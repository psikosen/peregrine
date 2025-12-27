package provider

import (
	"testing"

	"github.com/peregrine/router/internal/model"
)

func TestBaseProvider(t *testing.T) {
	caps := []model.Capability{model.CapModeling, model.CapSolving}
	bp := NewBaseProvider("test", 1, caps)

	t.Run("Name", func(t *testing.T) {
		if bp.Name() != "test" {
			t.Errorf("expected name 'test', got '%s'", bp.Name())
		}
	})

	t.Run("Priority", func(t *testing.T) {
		if bp.Priority() != 1 {
			t.Errorf("expected priority 1, got %d", bp.Priority())
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		if len(bp.Capabilities()) != 2 {
			t.Errorf("expected 2 capabilities, got %d", len(bp.Capabilities()))
		}
	})

	t.Run("SupportsCapability", func(t *testing.T) {
		if !bp.SupportsCapability(model.CapModeling) {
			t.Error("should support CapModeling")
		}
		if bp.SupportsCapability(model.CapComplexPlanning) {
			t.Error("should NOT support CapComplexPlanning")
		}
	})

	t.Run("Availability", func(t *testing.T) {
		if !bp.IsAvailable() {
			t.Error("should be available by default")
		}

		bp.SetAvailable(false)
		if bp.IsAvailable() {
			t.Error("should be unavailable after SetAvailable(false)")
		}

		bp.SetAvailable(true)
		if !bp.IsAvailable() {
			t.Error("should be available after SetAvailable(true)")
		}
	})
}

func TestProviderError(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := &ProviderError{Provider: "inner", Message: "inner error"}
		err := NewProviderError("outer", "outer error", cause)

		expected := "outer: outer error: inner: inner error"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}

		if err.Unwrap() != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("without cause", func(t *testing.T) {
		err := NewProviderError("test", "test error", nil)

		expected := "test: test error"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}

		if err.Unwrap() != nil {
			t.Error("Unwrap should return nil when no cause")
		}
	})
}
