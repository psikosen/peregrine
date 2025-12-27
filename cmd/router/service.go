package main

import (
	"context"
	"time"

	"github.com/peregrine/router/internal/metrics"
	"github.com/peregrine/router/internal/model"
)

// Complete handles LLM completion requests
func (s *LLMRouterService) Complete(ctx context.Context, req *CompleteRequest) (*CompleteResponse, error) {
	start := time.Now()

	// Convert proto request to internal model
	internalReq := &model.Request{
		AgentRole:            req.AgentRole,
		SystemPrompt:         req.SystemPrompt,
		UserPrompt:           req.UserPrompt,
		Temperature:          req.Temperature,
		MaxTokens:            int(req.MaxTokens),
		Metadata:             req.Metadata,
		RequiredCapabilities: model.CapabilitiesFromStrings(req.RequiredCapabilities),
	}

	// Route the request
	resp, err := s.router.Route(ctx, internalReq)
	if err != nil {
		metrics.RecordRequest("error", req.AgentRole, "error", time.Since(start).Seconds())
		return nil, err
	}

	// Record metrics
	metrics.RecordRequest(resp.Provider, req.AgentRole, "success", time.Since(start).Seconds())

	// Convert to proto response
	return &CompleteResponse{
		Content:    resp.Content,
		Provider:   resp.Provider,
		Confidence: resp.Confidence,
		TokensUsed: int32(resp.TokensUsed),
		Cached:     resp.Cached,
		LatencyMs:  resp.LatencyMs,
	}, nil
}

// GetHealth returns the health status of all providers
func (s *LLMRouterService) GetHealth(ctx context.Context, req *HealthRequest) (*HealthResponse, error) {
	statuses := s.router.GetProviders()

	protoHealth := make([]*ProviderHealth, len(statuses))
	for i, status := range statuses {
		protoHealth[i] = &ProviderHealth{
			Name:          status.Name,
			Available:     status.Available,
			FailureCount:  int32(status.FailureCount),
			LastCheckMs:   time.Now().UnixMilli(),
			StatusMessage: "",
		}
	}

	return &HealthResponse{
		Providers: protoHealth,
	}, nil
}

// ListProviders returns available providers and their capabilities
func (s *LLMRouterService) ListProviders(ctx context.Context, req *ListProvidersRequest) (*ListProvidersResponse, error) {
	statuses := s.router.GetProviders()

	protoProviders := make([]*ProviderInfo, len(statuses))
	for i, status := range statuses {
		protoProviders[i] = &ProviderInfo{
			Name:         status.Name,
			Priority:     int32(status.Priority),
			Capabilities: status.Capabilities,
			Available:    status.Available,
		}
	}

	return &ListProvidersResponse{
		Providers: protoProviders,
	}, nil
}
