package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/peregrine/router/internal/model"
	"github.com/peregrine/router/internal/provider"
)

// TestIntegration_FullFlow tests the complete request flow through the router
func TestIntegration_FullFlow(t *testing.T) {
	// Create a mock Ollama server
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"models": [{"name": "gemma3:270m"}]}`))
			return
		}

		if r.URL.Path == "/api/generate" {
			// Return a structured solver response
			response := map[string]interface{}{
				"model": "gemma3:270m",
				"response": `{
					"action": "PATCH_RESOURCE",
					"target": "auth-service",
					"patch": {"resources.limits.memory": "512Mi"},
					"justification": ["Observed OOMKilled", "Increasing memory limit"],
					"risk": "LOW",
					"confidence": 0.92
				}`,
				"done":              true,
				"prompt_eval_count": 100,
				"eval_count":        50,
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ollamaServer.Close()

	// Create providers
	ollamaCfg := provider.OllamaConfig{
		Endpoint:           ollamaServer.URL,
		Model:              "gemma3:270m",
		Timeout:            10 * time.Second,
		ConfidenceThreshold: 0.7,
	}
	ollamaProvider := provider.NewOllamaProvider(ollamaCfg)

	rulesCfg := provider.DefaultRulesConfig()
	rulesProvider := provider.NewRulesProvider(rulesCfg)

	// Create router with Ollama and Rules
	r := NewRouter(
		[]provider.Provider{ollamaProvider, rulesProvider},
		DefaultHealthConfig(),
	)

	t.Run("solver request to Ollama", func(t *testing.T) {
		// Create a solver request (Modeler output as input)
		modelerOutput := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{
					"name": "auth-service",
					"type": "k8s-pod",
				},
			},
			"state_variables": map[string]interface{}{
				"restart_count":      12,
				"termination_reason": "OOMKilled",
				"memory_limit":       "256Mi",
			},
			"constraints": []string{
				"memory_limit < observed_usage",
			},
			"confidence": 0.93,
		}
		modelerJSON, _ := json.Marshal(modelerOutput)

		req := &model.Request{
			AgentRole:    "solver",
			SystemPrompt: "You are a solver agent. Output JSON only.",
			UserPrompt:   string(modelerJSON),
			Temperature:  0.1,
			MaxTokens:    500,
			RequiredCapabilities: []model.Capability{model.CapSolving},
		}

		resp, err := r.Route(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify we got a response from Ollama
		if resp.Provider != "ollama" {
			t.Errorf("expected provider 'ollama', got '%s'", resp.Provider)
		}

		// Parse and verify the response
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result["action"] != "PATCH_RESOURCE" {
			t.Errorf("expected action 'PATCH_RESOURCE', got '%v'", result["action"])
		}

		if result["target"] != "auth-service" {
			t.Errorf("expected target 'auth-service', got '%v'", result["target"])
		}
	})

	t.Run("fallback to rules engine for OOMKilled", func(t *testing.T) {
		// Mark Ollama as unavailable to test fallback
		ollamaProvider.SetAvailable(false)

		modelerOutput := map[string]interface{}{
			"entities": []interface{}{
				map[string]interface{}{
					"name": "web-service",
					"type": "k8s-pod",
				},
			},
			"state_variables": map[string]interface{}{
				"termination_reason": "OOMKilled",
			},
		}
		modelerJSON, _ := json.Marshal(modelerOutput)

		req := &model.Request{
			AgentRole:  "solver",
			UserPrompt: string(modelerJSON),
			// No required capabilities - rules engine can handle
		}

		resp, err := r.Route(context.Background(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should fall back to rules engine
		if resp.Provider != "rules" {
			t.Errorf("expected provider 'rules', got '%s'", resp.Provider)
		}

		// Rules engine should have 100% confidence
		if resp.Confidence != 1.0 {
			t.Errorf("expected confidence 1.0 from rules, got %v", resp.Confidence)
		}

		// Restore Ollama
		ollamaProvider.SetAvailable(true)
	})
}

// TestIntegration_CapabilityDowngrade tests that complex planning requires external LLM
func TestIntegration_CapabilityDowngrade(t *testing.T) {
	// Only Ollama provider (no external LLM)
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			response := map[string]interface{}{
				"model":    "gemma3:270m",
				"response": `{"result": "ok"}`,
				"done":     true,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaServer.Close()

	ollamaCfg := provider.OllamaConfig{
		Endpoint: ollamaServer.URL,
		Model:    "gemma3:270m",
		Timeout:  10 * time.Second,
	}
	ollamaProvider := provider.NewOllamaProvider(ollamaCfg)

	r := NewRouter([]provider.Provider{ollamaProvider}, DefaultHealthConfig())

	// Request complex planning - Ollama doesn't support this
	req := &model.Request{
		AgentRole:            "solver",
		UserPrompt:           "complex multi-step plan",
		RequiredCapabilities: []model.Capability{model.CapComplexPlanning},
	}

	_, err := r.Route(context.Background(), req)
	if err != ErrCapabilityNotSupported {
		t.Errorf("expected ErrCapabilityNotSupported, got %v", err)
	}
}

// TestIntegration_HealthCheck tests the health management
func TestIntegration_HealthCheck(t *testing.T) {
	// Create a server that will fail after first request
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount > 1 {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		response := map[string]interface{}{
			"model":    "gemma3:270m",
			"response": `{"result": "ok", "confidence": 0.9}`,
			"done":     true,
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	cfg := provider.OllamaConfig{
		Endpoint:           server.URL,
		Model:              "gemma3:270m",
		Timeout:            10 * time.Second,
		ConfidenceThreshold: 0.7,
	}
	ollamaProvider := provider.NewOllamaProvider(cfg)

	// Use a short health config for testing
	healthCfg := HealthConfig{
		Interval:         100 * time.Millisecond,
		FailureThreshold: 2,
		RecoveryTime:     500 * time.Millisecond,
	}

	r := NewRouter([]provider.Provider{ollamaProvider}, healthCfg)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	// First request should succeed
	_, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}

	// Second request will fail (server returns error)
	_, err = r.Route(context.Background(), req)
	if err == nil {
		t.Error("second request should fail")
	}
}
