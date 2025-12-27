package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/peregrine/router/internal/metrics"
	"github.com/peregrine/router/internal/model"
)

// NgrokConfig holds ngrok AI Gateway configuration
type NgrokConfig struct {
	Endpoint string        `yaml:"endpoint"`
	APIKey   string        `yaml:"api_key"`
	Timeout  time.Duration `yaml:"timeout"`
	Model    string        `yaml:"model"` // e.g., "claude-3-opus", "gemini-pro"
}

// DefaultNgrokConfig returns sensible defaults
func DefaultNgrokConfig() NgrokConfig {
	return NgrokConfig{
		Endpoint: "https://ai-gateway.ngrok.io",
		Timeout:  30 * time.Second,
		Model:    "claude-3-sonnet",
	}
}

// NgrokProvider implements the Provider interface for ngrok AI Gateway
type NgrokProvider struct {
	*BaseProvider
	config NgrokConfig
	client *http.Client
}

// NewNgrokProvider creates a new ngrok provider
func NewNgrokProvider(config NgrokConfig) *NgrokProvider {
	return &NgrokProvider{
		BaseProvider: NewBaseProvider("ngrok", 1, model.AllCapabilities()),
		config:       config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ngrokRequest represents the ngrok AI Gateway request format
// Uses OpenAI-compatible format
type ngrokRequest struct {
	Model       string          `json:"model"`
	Messages    []ngrokMessage  `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type ngrokMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ngrokResponse represents the ngrok AI Gateway response
type ngrokResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int  `json:"prompt_tokens"`
		CompletionTokens int  `json:"completion_tokens"`
		TotalTokens      int  `json:"total_tokens"`
		CacheHit         bool `json:"cache_hit,omitempty"`
		CachedTokens     int  `json:"cached_tokens,omitempty"`
	} `json:"usage"`
}

// Send executes a completion request through ngrok AI Gateway
func (p *NgrokProvider) Send(ctx context.Context, req *model.Request) (*model.Response, error) {
	start := time.Now()

	// Build messages array
	messages := []ngrokMessage{
		{Role: "system", Content: req.SystemPrompt},
		{Role: "user", Content: req.UserPrompt},
	}

	// Prepare the request
	ngrokReq := ngrokRequest{
		Model:       p.config.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	body, err := json.Marshal(ngrokReq)
	if err != nil {
		return nil, NewProviderError(p.Name(), "failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		p.config.Endpoint+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, NewProviderError(p.Name(), "failed to create request", err)
	}

	// Set headers for prompt caching
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	httpReq.Header.Set("X-Agent-Role", req.AgentRole) // Used for cache key

	// Execute request
	metrics.ActiveRequests.WithLabelValues(p.Name()).Inc()
	defer metrics.ActiveRequests.WithLabelValues(p.Name()).Dec()

	resp, err := p.client.Do(httpReq)
	if err != nil {
		p.SetAvailable(false)
		metrics.RecordProviderFailure(p.Name())
		return nil, NewProviderError(p.Name(), "request failed", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 500 {
			p.SetAvailable(false)
		}
		metrics.RecordProviderFailure(p.Name())
		return nil, NewProviderError(p.Name(),
			fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	// Parse response
	var ngrokResp ngrokResponse
	if err := json.NewDecoder(resp.Body).Decode(&ngrokResp); err != nil {
		return nil, NewProviderError(p.Name(), "failed to decode response", err)
	}

	latency := time.Since(start)

	// Extract content
	content := ""
	if len(ngrokResp.Choices) > 0 {
		content = ngrokResp.Choices[0].Message.Content
	}

	// Extract confidence from response
	confidence := p.extractConfidence(content)

	// Track cache metrics
	if ngrokResp.Usage.CacheHit {
		metrics.RecordCacheHit(req.AgentRole, ngrokResp.Usage.CachedTokens)
	}

	// Build response
	response := &model.Response{
		Content:    content,
		Provider:   p.Name(),
		Confidence: confidence,
		TokensUsed: ngrokResp.Usage.TotalTokens,
		Cached:     ngrokResp.Usage.CacheHit,
		LatencyMs:  latency.Milliseconds(),
	}

	return response, nil
}

// extractConfidence attempts to parse confidence from the model's response
func (p *NgrokProvider) extractConfidence(response string) float64 {
	var parsed struct {
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err == nil && parsed.Confidence > 0 {
		return parsed.Confidence
	}

	// External LLMs generally have high confidence for structured output
	if len(response) > 0 && (response[0] == '{' || response[0] == '[') {
		return 0.9
	}

	return 0.7
}

// HealthCheck verifies ngrok AI Gateway is reachable
func (p *NgrokProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/health", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		p.SetAvailable(false)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		p.SetAvailable(false)
		return fmt.Errorf("health check failed: status %d", resp.StatusCode)
	}

	p.SetAvailable(true)
	return nil
}
