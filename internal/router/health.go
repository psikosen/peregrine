package router

import (
	"context"
	"sync"
	"time"

	"github.com/peregrine/router/internal/metrics"
	"github.com/peregrine/router/internal/provider"
)

// HealthConfig holds health checking configuration
type HealthConfig struct {
	Interval         time.Duration `yaml:"interval"`
	FailureThreshold int           `yaml:"failure_threshold"`
	RecoveryTime     time.Duration `yaml:"recovery_time"`
}

// DefaultHealthConfig returns sensible defaults
func DefaultHealthConfig() HealthConfig {
	return HealthConfig{
		Interval:         10 * time.Second,
		FailureThreshold: 3,
		RecoveryTime:     30 * time.Second,
	}
}

// HealthManager manages provider health checks
type HealthManager struct {
	providers  []provider.Provider
	config     HealthConfig
	failures   map[string]*providerState
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

type providerState struct {
	failureCount    int
	lastFailure     time.Time
	unavailableSince time.Time
	inRecovery      bool
}

// NewHealthManager creates a new health manager
func NewHealthManager(providers []provider.Provider, config HealthConfig) *HealthManager {
	failures := make(map[string]*providerState)
	for _, p := range providers {
		failures[p.Name()] = &providerState{}
	}

	return &HealthManager{
		providers: providers,
		config:    config,
		failures:  failures,
	}
}

// Start begins background health checking
func (h *HealthManager) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)

	h.wg.Add(1)
	go h.healthCheckLoop()
}

// Stop halts health checking
func (h *HealthManager) Stop() {
	if h.cancel != nil {
		h.cancel()
	}
	h.wg.Wait()
}

// healthCheckLoop periodically checks all providers
func (h *HealthManager) healthCheckLoop() {
	defer h.wg.Done()

	ticker := time.NewTicker(h.config.Interval)
	defer ticker.Stop()

	// Initial check
	h.checkAllProviders()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.checkAllProviders()
		}
	}
}

// checkAllProviders runs health checks on all providers
func (h *HealthManager) checkAllProviders() {
	var wg sync.WaitGroup

	for _, p := range h.providers {
		wg.Add(1)
		go func(p provider.Provider) {
			defer wg.Done()
			h.checkProvider(p)
		}(p)
	}

	wg.Wait()
}

// checkProvider runs a health check on a single provider
func (h *HealthManager) checkProvider(p provider.Provider) {
	h.mu.Lock()
	state := h.failures[p.Name()]
	h.mu.Unlock()

	// Skip if in recovery period
	if state.inRecovery && time.Since(state.unavailableSince) < h.config.RecoveryTime {
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()

	// Check if provider has a health check method
	type healthChecker interface {
		HealthCheck(context.Context) error
	}

	if hc, ok := p.(healthChecker); ok {
		err := hc.HealthCheck(ctx)
		h.mu.Lock()
		if err != nil {
			h.recordFailureLocked(p.Name())
		} else {
			h.recordSuccessLocked(p.Name())
		}
		h.mu.Unlock()
	}

	// Update metrics
	metrics.RecordProviderStatus(p.Name(), p.IsAvailable())
}

// RecordFailure records a provider failure
func (h *HealthManager) RecordFailure(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recordFailureLocked(name)
}

func (h *HealthManager) recordFailureLocked(name string) {
	state := h.failures[name]
	if state == nil {
		return
	}

	state.failureCount++
	state.lastFailure = time.Now()

	// Apply circuit breaker
	if state.failureCount >= h.config.FailureThreshold {
		state.inRecovery = true
		state.unavailableSince = time.Now()

		// Find provider and mark as unavailable
		for _, p := range h.providers {
			if p.Name() == name {
				if bp, ok := p.(*provider.OllamaProvider); ok {
					bp.SetAvailable(false)
				}
				if bp, ok := p.(*provider.NgrokProvider); ok {
					bp.SetAvailable(false)
				}
				break
			}
		}
	}

	metrics.RecordProviderFailure(name)
}

// RecordSuccess records a successful request
func (h *HealthManager) RecordSuccess(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.recordSuccessLocked(name)
}

func (h *HealthManager) recordSuccessLocked(name string) {
	state := h.failures[name]
	if state == nil {
		return
	}

	// Reset on success
	state.failureCount = 0
	state.inRecovery = false

	// Mark as available
	for _, p := range h.providers {
		if p.Name() == name {
			if bp, ok := p.(*provider.OllamaProvider); ok {
				bp.SetAvailable(true)
			}
			if bp, ok := p.(*provider.NgrokProvider); ok {
				bp.SetAvailable(true)
			}
			break
		}
	}
}

// GetFailureCount returns the current failure count for a provider
func (h *HealthManager) GetFailureCount(name string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if state, ok := h.failures[name]; ok {
		return state.failureCount
	}
	return 0
}

// GetStatus returns the health status of all providers
func (h *HealthManager) GetStatus() map[string]ProviderHealth {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]ProviderHealth)
	for _, p := range h.providers {
		state := h.failures[p.Name()]
		status[p.Name()] = ProviderHealth{
			Name:         p.Name(),
			Available:    p.IsAvailable(),
			FailureCount: state.failureCount,
			InRecovery:   state.inRecovery,
			LastFailure:  state.lastFailure,
		}
	}

	return status
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Name         string
	Available    bool
	FailureCount int
	InRecovery   bool
	LastFailure  time.Time
}
