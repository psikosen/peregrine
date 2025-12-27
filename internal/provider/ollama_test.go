package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/peregrine/router/internal/model"
)

func TestOllamaProvider_Send(t *testing.T) {
	// Create a mock Ollama server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse request
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify request fields
		if req.Model != "gemma3:270m" {
			t.Errorf("expected model 'gemma3:270m', got '%s'", req.Model)
		}

		if req.Stream != false {
			t.Error("expected stream to be false")
		}

		// Return mock response
		resp := ollamaResponse{
			Model:           "gemma3:270m",
			Response:        `{"action": "PATCH_RESOURCE", "confidence": 0.85}`,
			Done:            true,
			TotalDuration:   1000000000,
			PromptEvalCount: 50,
			EvalCount:       30,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create provider with mock server
	cfg := OllamaConfig{
		Endpoint:           server.URL,
		Model:              "gemma3:270m",
		Timeout:            10 * time.Second,
		ConfidenceThreshold: 0.7,
	}

	p := NewOllamaProvider(cfg)

	req := &model.Request{
		AgentRole:    "solver",
		SystemPrompt: "You are a solver agent.",
		UserPrompt:   `{"entities": [{"name": "test"}]}`,
		Temperature:  0.1,
		MaxTokens:    500,
	}

	resp, err := p.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", resp.Provider)
	}

	if resp.TokensUsed != 80 { // 50 + 30
		t.Errorf("expected 80 tokens, got %d", resp.TokensUsed)
	}

	// Should extract confidence from JSON response
	if resp.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %v", resp.Confidence)
	}
}

func TestOllamaProvider_Send_LowConfidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollamaResponse{
			Model:    "gemma3:270m",
			Response: `{"confidence": 0.3}`, // Below threshold
			Done:     true,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := OllamaConfig{
		Endpoint:           server.URL,
		Model:              "gemma3:270m",
		Timeout:            10 * time.Second,
		ConfidenceThreshold: 0.7,
	}

	p := NewOllamaProvider(cfg)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	resp, err := p.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return unsure response
	if resp.Confidence != 0 {
		t.Errorf("expected confidence 0 (unsure), got %v", resp.Confidence)
	}
}

func TestOllamaProvider_Send_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := OllamaConfig{
		Endpoint: server.URL,
		Model:    "gemma3:270m",
		Timeout:  10 * time.Second,
	}

	p := NewOllamaProvider(cfg)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	_, err := p.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error for server error response")
	}

	// Provider should be marked unavailable
	if p.IsAvailable() {
		t.Error("provider should be marked unavailable after server error")
	}
}

func TestOllamaProvider_Send_UnsupportedCapability(t *testing.T) {
	cfg := DefaultOllamaConfig()
	p := NewOllamaProvider(cfg)

	req := &model.Request{
		AgentRole:            "solver",
		UserPrompt:           "test",
		RequiredCapabilities: []model.Capability{model.CapComplexPlanning},
	}

	_, err := p.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error for unsupported capability")
	}

	providerErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}

	if providerErr.Provider != "ollama" {
		t.Errorf("expected provider 'ollama', got '%s'", providerErr.Provider)
	}
}

func TestOllamaProvider_HealthCheck(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/tags" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"models": []}`))
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer server.Close()

		cfg := OllamaConfig{
			Endpoint: server.URL,
			Timeout:  10 * time.Second,
		}

		p := NewOllamaProvider(cfg)

		err := p.HealthCheck(context.Background())
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !p.IsAvailable() {
			t.Error("provider should be available after successful health check")
		}
	})

	t.Run("unhealthy", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
		}))
		defer server.Close()

		cfg := OllamaConfig{
			Endpoint: server.URL,
			Timeout:  10 * time.Second,
		}

		p := NewOllamaProvider(cfg)

		err := p.HealthCheck(context.Background())
		if err == nil {
			t.Error("expected error for unhealthy server")
		}

		if p.IsAvailable() {
			t.Error("provider should be unavailable after failed health check")
		}
	})
}

func TestOllamaProvider_Capabilities(t *testing.T) {
	cfg := DefaultOllamaConfig()
	p := NewOllamaProvider(cfg)

	caps := p.Capabilities()

	// Should support basic capabilities
	if !model.HasCapability(caps, model.CapModeling) {
		t.Error("should support CapModeling")
	}

	if !model.HasCapability(caps, model.CapSolving) {
		t.Error("should support CapSolving")
	}

	// Should NOT support complex capabilities
	if model.HasCapability(caps, model.CapComplexPlanning) {
		t.Error("should NOT support CapComplexPlanning")
	}
}
