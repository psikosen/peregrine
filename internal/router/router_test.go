package router

import (
	"context"
	"errors"
	"testing"

	"github.com/peregrine/router/internal/model"
	"github.com/peregrine/router/internal/provider"
)

// mockProvider implements provider.Provider for testing
type mockProvider struct {
	name         string
	priority     int
	capabilities []model.Capability
	available    bool
	sendFunc     func(ctx context.Context, req *model.Request) (*model.Response, error)
	sendCount    int
}

func newMockProvider(name string, priority int, caps []model.Capability) *mockProvider {
	return &mockProvider{
		name:         name,
		priority:     priority,
		capabilities: caps,
		available:    true,
	}
}

func (m *mockProvider) Name() string                        { return m.name }
func (m *mockProvider) Priority() int                       { return m.priority }
func (m *mockProvider) Capabilities() []model.Capability    { return m.capabilities }
func (m *mockProvider) IsAvailable() bool                   { return m.available }
func (m *mockProvider) SupportsCapability(c model.Capability) bool {
	return model.HasCapability(m.capabilities, c)
}

func (m *mockProvider) Send(ctx context.Context, req *model.Request) (*model.Response, error) {
	m.sendCount++
	if m.sendFunc != nil {
		return m.sendFunc(ctx, req)
	}
	return &model.Response{
		Content:    `{"result": "ok"}`,
		Provider:   m.name,
		Confidence: 0.9,
		TokensUsed: 100,
	}, nil
}

func TestRouter_Route_PriorityOrder(t *testing.T) {
	// Create providers with different priorities
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	fallback := newMockProvider("fallback", 2, model.AllCapabilities())

	r := NewRouter([]provider.Provider{fallback, primary}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use primary (lower priority number = higher priority)
	if resp.Provider != "primary" {
		t.Errorf("expected provider 'primary', got '%s'", resp.Provider)
	}

	if primary.sendCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.sendCount)
	}

	if fallback.sendCount != 0 {
		t.Errorf("expected fallback to NOT be called, got %d", fallback.sendCount)
	}
}

func TestRouter_Route_Fallback(t *testing.T) {
	// Primary fails, fallback succeeds
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	primary.sendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return nil, errors.New("primary failed")
	}

	fallback := newMockProvider("fallback", 2, model.AllCapabilities())

	r := NewRouter([]provider.Provider{primary, fallback}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to secondary
	if resp.Provider != "fallback" {
		t.Errorf("expected provider 'fallback', got '%s'", resp.Provider)
	}

	if primary.sendCount != 1 {
		t.Errorf("expected primary to be called once, got %d", primary.sendCount)
	}

	if fallback.sendCount != 1 {
		t.Errorf("expected fallback to be called once, got %d", fallback.sendCount)
	}
}

func TestRouter_Route_CapabilityFiltering(t *testing.T) {
	// Primary only supports basic capabilities
	primary := newMockProvider("primary", 1, model.BasicCapabilities())

	// Fallback supports all capabilities
	fallback := newMockProvider("fallback", 2, model.AllCapabilities())

	r := NewRouter([]provider.Provider{primary, fallback}, DefaultHealthConfig())

	// Request requiring complex planning
	req := &model.Request{
		AgentRole:            "solver",
		UserPrompt:           "test",
		RequiredCapabilities: []model.Capability{model.CapComplexPlanning},
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip primary (doesn't support complex_planning) and use fallback
	if resp.Provider != "fallback" {
		t.Errorf("expected provider 'fallback', got '%s'", resp.Provider)
	}

	// Primary should not be called
	if primary.sendCount != 0 {
		t.Errorf("expected primary to NOT be called, got %d", primary.sendCount)
	}
}

func TestRouter_Route_NoCapability(t *testing.T) {
	// Only provider that doesn't support required capability
	primary := newMockProvider("primary", 1, model.BasicCapabilities())

	r := NewRouter([]provider.Provider{primary}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:            "solver",
		UserPrompt:           "test",
		RequiredCapabilities: []model.Capability{model.CapComplexPlanning},
	}

	_, err := r.Route(context.Background(), req)
	if err != ErrCapabilityNotSupported {
		t.Errorf("expected ErrCapabilityNotSupported, got %v", err)
	}
}

func TestRouter_Route_UnavailableProvider(t *testing.T) {
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	primary.available = false

	fallback := newMockProvider("fallback", 2, model.AllCapabilities())

	r := NewRouter([]provider.Provider{primary, fallback}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should skip unavailable primary
	if resp.Provider != "fallback" {
		t.Errorf("expected provider 'fallback', got '%s'", resp.Provider)
	}

	if primary.sendCount != 0 {
		t.Errorf("expected primary to NOT be called, got %d", primary.sendCount)
	}
}

func TestRouter_Route_UnsureResponse(t *testing.T) {
	// Primary returns unsure, fallback succeeds
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	primary.sendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return model.UnsureResponse("primary", "confidence too low"), nil
	}

	fallback := newMockProvider("fallback", 2, model.AllCapabilities())

	r := NewRouter([]provider.Provider{primary, fallback}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back when primary is unsure
	if resp.Provider != "fallback" {
		t.Errorf("expected provider 'fallback', got '%s'", resp.Provider)
	}
}

func TestRouter_Route_AllFail(t *testing.T) {
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	primary.sendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return nil, errors.New("primary failed")
	}

	fallback := newMockProvider("fallback", 2, model.AllCapabilities())
	fallback.sendFunc = func(ctx context.Context, req *model.Request) (*model.Response, error) {
		return nil, errors.New("fallback failed")
	}

	r := NewRouter([]provider.Provider{primary, fallback}, DefaultHealthConfig())

	req := &model.Request{
		AgentRole:  "solver",
		UserPrompt: "test",
	}

	_, err := r.Route(context.Background(), req)
	if err == nil {
		t.Error("expected error when all providers fail")
	}
}

func TestRouter_GetProviders(t *testing.T) {
	primary := newMockProvider("primary", 1, model.AllCapabilities())
	fallback := newMockProvider("fallback", 2, model.BasicCapabilities())

	r := NewRouter([]provider.Provider{fallback, primary}, DefaultHealthConfig())

	statuses := r.GetProviders()

	if len(statuses) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(statuses))
	}

	// Should be sorted by priority
	if statuses[0].Name != "primary" {
		t.Errorf("expected first provider to be 'primary', got '%s'", statuses[0].Name)
	}

	if statuses[1].Name != "fallback" {
		t.Errorf("expected second provider to be 'fallback', got '%s'", statuses[1].Name)
	}
}

func TestRouter_NoRequiredCapabilities(t *testing.T) {
	primary := newMockProvider("primary", 1, model.BasicCapabilities())

	r := NewRouter([]provider.Provider{primary}, DefaultHealthConfig())

	// No required capabilities - should use any provider
	req := &model.Request{
		AgentRole:            "solver",
		UserPrompt:           "test",
		RequiredCapabilities: []model.Capability{}, // empty
	}

	resp, err := r.Route(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Provider != "primary" {
		t.Errorf("expected provider 'primary', got '%s'", resp.Provider)
	}
}
