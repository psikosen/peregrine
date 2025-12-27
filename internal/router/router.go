package router

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/peregrine/router/internal/metrics"
	"github.com/peregrine/router/internal/model"
	"github.com/peregrine/router/internal/provider"
)

var (
	// ErrNoAvailableProvider indicates no provider could handle the request
	ErrNoAvailableProvider = errors.New("no available provider for request")

	// ErrCapabilityNotSupported indicates required capability is not available
	ErrCapabilityNotSupported = errors.New("required capability not supported by any provider")
)

// Router handles LLM request routing with priority-based fallback
type Router struct {
	providers []provider.Provider
	health    *HealthManager
	mu        sync.RWMutex
}

// NewRouter creates a new router with the given providers
func NewRouter(providers []provider.Provider, healthConfig HealthConfig) *Router {
	// Sort providers by priority (lower = higher priority)
	sorted := make([]provider.Provider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})

	r := &Router{
		providers: sorted,
	}

	// Initialize health manager
	r.health = NewHealthManager(providers, healthConfig)

	return r
}

// Route sends a request to the best available provider
func (r *Router) Route(ctx context.Context, req *model.Request) (*model.Response, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find providers that support required capabilities
	eligible := r.findEligibleProviders(req.RequiredCapabilities)
	if len(eligible) == 0 {
		metrics.RecordRoutingDecision("no_capability", "none")
		return nil, ErrCapabilityNotSupported
	}

	// Try providers in priority order
	var lastErr error
	for _, p := range eligible {
		if !p.IsAvailable() {
			continue
		}

		resp, err := p.Send(ctx, req)
		if err != nil {
			lastErr = err
			metrics.RecordProviderFailure(p.Name())
			r.health.RecordFailure(p.Name())
			continue
		}

		// Check for unsure response
		if resp.Confidence == 0 {
			metrics.RecordRoutingDecision("unsure", p.Name())
			// Continue to next provider if this one is unsure
			lastErr = errors.New("provider unsure: " + resp.Content)
			continue
		}

		// Success
		metrics.RecordRoutingDecision("success", p.Name())
		metrics.RecordConfidence(p.Name(), req.AgentRole, resp.Confidence)
		r.health.RecordSuccess(p.Name())

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}

	metrics.RecordRoutingDecision("no_available", "none")
	return nil, ErrNoAvailableProvider
}

// findEligibleProviders returns providers that support all required capabilities
func (r *Router) findEligibleProviders(required []model.Capability) []provider.Provider {
	if len(required) == 0 {
		// No specific requirements, all providers are eligible
		return r.providers
	}

	eligible := make([]provider.Provider, 0)
	for _, p := range r.providers {
		supports := true
		for _, cap := range required {
			if !p.SupportsCapability(cap) {
				supports = false
				break
			}
		}
		if supports {
			eligible = append(eligible, p)
		}
	}

	return eligible
}

// GetProviders returns all registered providers with their status
func (r *Router) GetProviders() []ProviderStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]ProviderStatus, len(r.providers))
	for i, p := range r.providers {
		statuses[i] = ProviderStatus{
			Name:         p.Name(),
			Priority:     p.Priority(),
			Available:    p.IsAvailable(),
			Capabilities: model.CapabilitiesToStrings(p.Capabilities()),
			FailureCount: r.health.GetFailureCount(p.Name()),
		}
	}

	return statuses
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	Name         string
	Priority     int
	Available    bool
	Capabilities []string
	FailureCount int
}

// Start begins background health checking
func (r *Router) Start(ctx context.Context) {
	r.health.Start(ctx)
}

// Stop shuts down the router
func (r *Router) Stop() {
	r.health.Stop()
}
