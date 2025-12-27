package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/peregrine/router/internal/model"
)

func TestRulesProvider_OOMKilled(t *testing.T) {
	cfg := DefaultRulesConfig()
	p := NewRulesProvider(cfg)

	// Simulate a Modeler output for OOMKilled scenario
	input := map[string]interface{}{
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

	inputJSON, _ := json.Marshal(input)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: string(inputJSON),
	}

	resp, err := p.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Verify response structure
	if result["action"] != "PATCH_RESOURCE" {
		t.Errorf("expected action 'PATCH_RESOURCE', got '%v'", result["action"])
	}

	if result["target"] != "auth-service" {
		t.Errorf("expected target 'auth-service', got '%v'", result["target"])
	}

	if result["risk"] != "LOW" {
		t.Errorf("expected risk 'LOW', got '%v'", result["risk"])
	}

	if result["matched_rule"] != "oom-killed" {
		t.Errorf("expected matched_rule 'oom-killed', got '%v'", result["matched_rule"])
	}

	// Confidence should be 1.0 for deterministic rules
	if resp.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %v", resp.Confidence)
	}

	if resp.Provider != "rules" {
		t.Errorf("expected provider 'rules', got '%s'", resp.Provider)
	}
}

func TestRulesProvider_ImagePullBackOff(t *testing.T) {
	cfg := DefaultRulesConfig()
	p := NewRulesProvider(cfg)

	input := map[string]interface{}{
		"entities": []interface{}{
			map[string]interface{}{
				"name": "web-frontend",
				"type": "k8s-pod",
			},
		},
		"state_variables": map[string]interface{}{
			"status": "ImagePullBackOff",
		},
	}

	inputJSON, _ := json.Marshal(input)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: string(inputJSON),
	}

	resp, err := p.Send(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["action"] != "DIAGNOSE" {
		t.Errorf("expected action 'DIAGNOSE', got '%v'", result["action"])
	}

	if result["matched_rule"] != "image-pull-backoff" {
		t.Errorf("expected matched_rule 'image-pull-backoff', got '%v'", result["matched_rule"])
	}
}

func TestRulesProvider_NoMatch(t *testing.T) {
	cfg := DefaultRulesConfig()
	p := NewRulesProvider(cfg)

	// Input that doesn't match any rule
	input := map[string]interface{}{
		"entities": []interface{}{
			map[string]interface{}{
				"name": "healthy-service",
				"type": "k8s-pod",
			},
		},
		"state_variables": map[string]interface{}{
			"status": "Running",
		},
	}

	inputJSON, _ := json.Marshal(input)

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: string(inputJSON),
	}

	_, err := p.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error for no matching rule")
	}

	// Should be a provider error
	providerErr, ok := err.(*ProviderError)
	if !ok {
		t.Fatalf("expected ProviderError, got %T", err)
	}

	if providerErr.Provider != "rules" {
		t.Errorf("expected provider 'rules', got '%s'", providerErr.Provider)
	}
}

func TestRulesProvider_Disabled(t *testing.T) {
	cfg := DefaultRulesConfig()
	cfg.Enabled = false
	p := NewRulesProvider(cfg)

	req := &model.Request{
		UserPrompt: `{"status": "OOMKilled"}`,
	}

	_, err := p.Send(context.Background(), req)
	if err == nil {
		t.Error("expected error when rules engine is disabled")
	}
}

func TestRulesProvider_IsAvailable(t *testing.T) {
	t.Run("enabled", func(t *testing.T) {
		cfg := DefaultRulesConfig()
		cfg.Enabled = true
		p := NewRulesProvider(cfg)

		if !p.IsAvailable() {
			t.Error("should be available when enabled")
		}
	})

	t.Run("disabled", func(t *testing.T) {
		cfg := DefaultRulesConfig()
		cfg.Enabled = false
		p := NewRulesProvider(cfg)

		if p.IsAvailable() {
			t.Error("should not be available when disabled")
		}
	})
}
