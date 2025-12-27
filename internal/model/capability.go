package model

// Capability represents a specific LLM capability
type Capability string

const (
	// CapModeling - ability to build structured problem models from raw data
	CapModeling Capability = "modeling"

	// CapSolving - ability to produce actions from problem models
	CapSolving Capability = "solving"

	// CapComplexPlanning - ability to handle multi-step planning scenarios
	CapComplexPlanning Capability = "complex_planning"

	// CapMultiStepRemediation - ability to orchestrate complex remediation flows
	CapMultiStepRemediation Capability = "multi_step_remediation"
)

// AllCapabilities returns all defined capabilities
func AllCapabilities() []Capability {
	return []Capability{
		CapModeling,
		CapSolving,
		CapComplexPlanning,
		CapMultiStepRemediation,
	}
}

// BasicCapabilities returns capabilities that local models can handle
func BasicCapabilities() []Capability {
	return []Capability{
		CapModeling,
		CapSolving,
	}
}

// HasCapability checks if a capability slice contains a specific capability
func HasCapability(caps []Capability, target Capability) bool {
	for _, c := range caps {
		if c == target {
			return true
		}
	}
	return false
}

// CapabilitiesFromStrings converts string slice to Capability slice
func CapabilitiesFromStrings(strs []string) []Capability {
	caps := make([]Capability, len(strs))
	for i, s := range strs {
		caps[i] = Capability(s)
	}
	return caps
}

// CapabilitiesToStrings converts Capability slice to string slice
func CapabilitiesToStrings(caps []Capability) []string {
	strs := make([]string, len(caps))
	for i, c := range caps {
		strs[i] = string(c)
	}
	return strs
}
