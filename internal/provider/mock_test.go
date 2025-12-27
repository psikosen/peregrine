package provider

import (
	"context"
	"errors"

	"github.com/peregrine/router/internal/model"
)

// MockProvider is a test provider with configurable behavior
type MockProvider struct {
	*BaseProvider
	SendFunc      func(ctx context.Context, req *model.Request) (*model.Response, error)
	HealthFunc    func(ctx context.Context) error
	sendCallCount int
}

// NewMockProvider creates a new mock provider
func NewMockProvider(name string, priority int, caps []model.Capability) *MockProvider {
	return &MockProvider{
		BaseProvider: NewBaseProvider(name, priority, caps),
	}
}

// Send calls the configured SendFunc or returns a default response
func (m *MockProvider) Send(ctx context.Context, req *model.Request) (*model.Response, error) {
	m.sendCallCount++

	if m.SendFunc != nil {
		return m.SendFunc(ctx, req)
	}

	// Default: return a successful response
	return &model.Response{
		Content:    `{"status": "ok"}`,
		Provider:   m.Name(),
		Confidence: 0.9,
		TokensUsed: 100,
		Cached:     false,
		LatencyMs:  50,
	}, nil
}

// HealthCheck calls the configured HealthFunc or returns nil
func (m *MockProvider) HealthCheck(ctx context.Context) error {
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

// SendCallCount returns how many times Send was called
func (m *MockProvider) SendCallCount() int {
	return m.sendCallCount
}

// Reset resets the call count
func (m *MockProvider) Reset() {
	m.sendCallCount = 0
}

// Helper functions for creating common mock behaviors

// MockSuccess returns a provider that always succeeds
func MockSuccess(name string, priority int) *MockProvider {
	return NewMockProvider(name, priority, model.AllCapabilities())
}

// MockFailure returns a provider that always fails
func MockFailure(name string, priority int) *MockProvider {
	m := NewMockProvider(name, priority, model.AllCapabilities())
	m.SendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return nil, errors.New("mock failure")
	}
	return m
}

// MockUnsure returns a provider that returns an unsure response
func MockUnsure(name string, priority int) *MockProvider {
	m := NewMockProvider(name, priority, model.AllCapabilities())
	m.SendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return model.UnsureResponse(name, "low confidence"), nil
	}
	return m
}

// MockWithCapabilities returns a provider with specific capabilities
func MockWithCapabilities(name string, priority int, caps []model.Capability) *MockProvider {
	return NewMockProvider(name, priority, caps)
}
