package model

import "testing"

func TestHasCapability(t *testing.T) {
	tests := []struct {
		name   string
		caps   []Capability
		target Capability
		want   bool
	}{
		{
			name:   "has capability",
			caps:   []Capability{CapModeling, CapSolving},
			target: CapModeling,
			want:   true,
		},
		{
			name:   "missing capability",
			caps:   []Capability{CapModeling, CapSolving},
			target: CapComplexPlanning,
			want:   false,
		},
		{
			name:   "empty list",
			caps:   []Capability{},
			target: CapModeling,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasCapability(tt.caps, tt.target)
			if got != tt.want {
				t.Errorf("HasCapability() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCapabilitiesFromStrings(t *testing.T) {
	input := []string{"modeling", "solving"}
	caps := CapabilitiesFromStrings(input)

	if len(caps) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(caps))
	}
	if caps[0] != CapModeling {
		t.Errorf("expected CapModeling, got %v", caps[0])
	}
	if caps[1] != CapSolving {
		t.Errorf("expected CapSolving, got %v", caps[1])
	}
}

func TestCapabilitiesToStrings(t *testing.T) {
	caps := []Capability{CapModeling, CapComplexPlanning}
	strs := CapabilitiesToStrings(caps)

	if len(strs) != 2 {
		t.Fatalf("expected 2 strings, got %d", len(strs))
	}
	if strs[0] != "modeling" {
		t.Errorf("expected 'modeling', got %v", strs[0])
	}
	if strs[1] != "complex_planning" {
		t.Errorf("expected 'complex_planning', got %v", strs[1])
	}
}

func TestAllCapabilities(t *testing.T) {
	caps := AllCapabilities()
	if len(caps) != 4 {
		t.Errorf("expected 4 capabilities, got %d", len(caps))
	}
}

func TestBasicCapabilities(t *testing.T) {
	caps := BasicCapabilities()
	if len(caps) != 2 {
		t.Errorf("expected 2 basic capabilities, got %d", len(caps))
	}

	// Should include modeling and solving
	if !HasCapability(caps, CapModeling) {
		t.Error("basic capabilities should include CapModeling")
	}
	if !HasCapability(caps, CapSolving) {
		t.Error("basic capabilities should include CapSolving")
	}

	// Should NOT include complex planning
	if HasCapability(caps, CapComplexPlanning) {
		t.Error("basic capabilities should NOT include CapComplexPlanning")
	}
}
