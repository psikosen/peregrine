package router

import (
	"github.com/peregrine/router/internal/model"
)

// CapabilityConfig defines capability-based routing rules
type CapabilityConfig struct {
	// OllamaAllowed lists capabilities that Ollama can handle
	OllamaAllowed []model.Capability `yaml:"ollama_allowed"`

	// OllamaRejected lists capabilities that must use external LLM
	OllamaRejected []model.Capability `yaml:"ollama_rejected"`

	// ConfidenceThresholds per capability
	ConfidenceThresholds map[model.Capability]float64 `yaml:"confidence_thresholds"`
}

// DefaultCapabilityConfig returns sensible defaults
func DefaultCapabilityConfig() CapabilityConfig {
	return CapabilityConfig{
		OllamaAllowed: []model.Capability{
			model.CapModeling,
			model.CapSolving,
		},
		OllamaRejected: []model.Capability{
			model.CapComplexPlanning,
			model.CapMultiStepRemediation,
		},
		ConfidenceThresholds: map[model.Capability]float64{
			model.CapModeling:             0.7,
			model.CapSolving:              0.7,
			model.CapComplexPlanning:      0.9,
			model.CapMultiStepRemediation: 0.9,
		},
	}
}

// RequiresExternalLLM checks if the required capabilities need an external LLM
func RequiresExternalLLM(required []model.Capability, config CapabilityConfig) bool {
	for _, cap := range required {
		if model.HasCapability(config.OllamaRejected, cap) {
			return true
		}
	}
	return false
}

// GetConfidenceThreshold returns the minimum confidence for a capability
func GetConfidenceThreshold(cap model.Capability, config CapabilityConfig) float64 {
	if threshold, ok := config.ConfidenceThresholds[cap]; ok {
		return threshold
	}
	return 0.7 // Default threshold
}

// ValidateCapabilities checks if required capabilities are valid
func ValidateCapabilities(required []string) ([]model.Capability, error) {
	caps := make([]model.Capability, len(required))
	for i, r := range required {
		caps[i] = model.Capability(r)
	}
	return caps, nil
}
