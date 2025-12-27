package provider

import (
	"context"

	"github.com/peregrine/router/internal/model"
)

// Provider defines the interface for LLM backends
type Provider interface {
	// Name returns the provider identifier
	Name() string

	// Priority returns the routing priority (lower = higher priority)
	Priority() int

	// Send executes a completion request
	Send(ctx context.Context, req *model.Request) (*model.Response, error)

	// IsAvailable checks if the provider is currently healthy
	IsAvailable() bool

	// Capabilities returns the list of supported capabilities
	Capabilities() []model.Capability

	// SupportsCapability checks if a specific capability is supported
	SupportsCapability(cap model.Capability) bool
}

// BaseProvider provides common functionality for providers
type BaseProvider struct {
	name         string
	priority     int
	capabilities []model.Capability
	available    bool
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(name string, priority int, caps []model.Capability) *BaseProvider {
	return &BaseProvider{
		name:         name,
		priority:     priority,
		capabilities: caps,
		available:    true,
	}
}

// Name returns the provider name
func (p *BaseProvider) Name() string {
	return p.name
}

// Priority returns the provider priority
func (p *BaseProvider) Priority() int {
	return p.priority
}

// Capabilities returns supported capabilities
func (p *BaseProvider) Capabilities() []model.Capability {
	return p.capabilities
}

// SupportsCapability checks if a capability is supported
func (p *BaseProvider) SupportsCapability(cap model.Capability) bool {
	return model.HasCapability(p.capabilities, cap)
}

// IsAvailable returns availability status
func (p *BaseProvider) IsAvailable() bool {
	return p.available
}

// SetAvailable updates availability status
func (p *BaseProvider) SetAvailable(available bool) {
	p.available = available
}

// ProviderError represents a provider-specific error
type ProviderError struct {
	Provider string
	Message  string
	Cause    error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return e.Provider + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Provider + ": " + e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProviderError creates a new provider error
func NewProviderError(provider, message string, cause error) *ProviderError {
	return &ProviderError{
		Provider: provider,
		Message:  message,
		Cause:    cause,
	}
}
