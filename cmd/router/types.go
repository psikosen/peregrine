package main

import "context"

// This file contains temporary type definitions that will be replaced
// by generated protobuf code after running `make proto`.
//
// To generate the actual types:
//   1. Install protoc and Go plugins
//   2. Run: make proto
//   3. Import the generated package instead

// CompleteRequest represents an LLM completion request
type CompleteRequest struct {
	AgentRole            string
	SystemPrompt         string
	UserPrompt           string
	Temperature          float64
	MaxTokens            int32
	Metadata             map[string]string
	RequiredCapabilities []string
}

// CompleteResponse represents an LLM completion response
type CompleteResponse struct {
	Content    string
	Provider   string
	Confidence float64
	TokensUsed int32
	Cached     bool
	LatencyMs  int64
}

// HealthRequest is the health check request
type HealthRequest struct{}

// HealthResponse contains provider health status
type HealthResponse struct {
	Providers []*ProviderHealth
}

// ProviderHealth represents health of a single provider
type ProviderHealth struct {
	Name          string
	Available     bool
	FailureCount  int32
	LastCheckMs   int64
	StatusMessage string
}

// ListProvidersRequest is the list providers request
type ListProvidersRequest struct{}

// ListProvidersResponse contains provider information
type ListProvidersResponse struct {
	Providers []*ProviderInfo
}

// ProviderInfo represents information about a provider
type ProviderInfo struct {
	Name         string
	Priority     int32
	Capabilities []string
	Available    bool
}

// LLMRouterServer is the server interface
type LLMRouterServer interface {
	Complete(context.Context, *CompleteRequest) (*CompleteResponse, error)
	GetHealth(context.Context, *HealthRequest) (*HealthResponse, error)
	ListProviders(context.Context, *ListProvidersRequest) (*ListProvidersResponse, error)
}

// UnimplementedLLMRouterServer provides default implementations
type UnimplementedLLMRouterServer struct{}

// RegisterLLMRouterServer registers the service with gRPC
// This is a placeholder - actual implementation comes from generated code
func RegisterLLMRouterServer(s interface{}, srv LLMRouterServer) {
	// Will be replaced by generated code
}
