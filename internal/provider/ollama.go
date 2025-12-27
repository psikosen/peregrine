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

// OllamaConfig holds Ollama provider configuration
type OllamaConfig struct {
	Endpoint           string        `yaml:"endpoint"`
	Model              string        `yaml:"model"`
	Timeout            time.Duration `yaml:"timeout"`
	ConfidenceThreshold float64      `yaml:"confidence_threshold"`
}

// DefaultOllamaConfig returns sensible defaults
func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		Endpoint:           "http://localhost:11434",
		Model:              "gemma3:270m",
		Timeout:            60 * time.Second,
		ConfidenceThreshold: 0.7,
	}
}

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	*BaseProvider
	config OllamaConfig
	client *http.Client
}

// NewOllamaProvider creates a new Ollama provider
func NewOllamaProvider(config OllamaConfig) *OllamaProvider {
	return &OllamaProvider{
		BaseProvider: NewBaseProvider("ollama", 2, model.BasicCapabilities()),
		config:       config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ollamaRequest represents the Ollama API request format
type ollamaRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	Stream      bool    `json:"stream"`
}

// ollamaResponse represents the Ollama API response format
type ollamaResponse struct {
	Model              string `json:"model"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
}

// Send executes a completion request against Ollama
func (p *OllamaProvider) Send(ctx context.Context, req *model.Request) (*model.Response, error) {
	start := time.Now()

	// Check capability requirements
	for _, cap := range req.RequiredCapabilities {
		if !p.SupportsCapability(cap) {
			return nil, NewProviderError(p.Name(), "unsupported capability: "+string(cap), nil)
		}
	}

	// Build the prompt (system + user)
	prompt := req.SystemPrompt + "\n\n" + req.UserPrompt

	// Prepare the request
	ollamaReq := ollamaRequest{
		Model:       p.config.Model,
		Prompt:      prompt,
		Temperature: req.Temperature,
		Stream:      false,
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, NewProviderError(p.Name(), "failed to marshal request", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		p.config.Endpoint+"/api/generate",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, NewProviderError(p.Name(), "failed to create request", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		p.SetAvailable(false)
		metrics.RecordProviderFailure(p.Name())
		return nil, NewProviderError(p.Name(),
			fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes)), nil)
	}

	// Parse response
	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, NewProviderError(p.Name(), "failed to decode response", err)
	}

	latency := time.Since(start)

	// Extract confidence from response if present (model-dependent)
	confidence := p.extractConfidence(ollamaResp.Response)

	// Build response
	response := &model.Response{
		Content:    ollamaResp.Response,
		Provider:   p.Name(),
		Confidence: confidence,
		TokensUsed: ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		Cached:     false,
		LatencyMs:  latency.Milliseconds(),
	}

	// Check confidence threshold
	if confidence < p.config.ConfidenceThreshold {
		return model.UnsureResponse(p.Name(), "confidence below threshold"), nil
	}

	return response, nil
}

// extractConfidence attempts to parse confidence from the model's response
// This is a heuristic - proper implementation depends on model output format
func (p *OllamaProvider) extractConfidence(response string) float64 {
	// Try to parse JSON response with confidence field
	var parsed struct {
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err == nil && parsed.Confidence > 0 {
		return parsed.Confidence
	}

	// Default to a moderate confidence for structured responses
	if len(response) > 0 && (response[0] == '{' || response[0] == '[') {
		return 0.8
	}

	return 0.5
}

// HealthCheck verifies Ollama is reachable
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.config.Endpoint+"/api/tags", nil)
	if err != nil {
		return err
	}

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
