package provider

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/peregrine/router/internal/model"
)

// RulesConfig holds deterministic rule engine configuration
type RulesConfig struct {
	Enabled bool   `yaml:"enabled"`
	Rules   []Rule `yaml:"rules"`
}

// Rule defines a deterministic remediation rule
type Rule struct {
	Name      string            `yaml:"name"`
	Condition RuleCondition     `yaml:"condition"`
	Action    RuleAction        `yaml:"action"`
	Priority  int               `yaml:"priority"`
}

// RuleCondition defines when a rule matches
type RuleCondition struct {
	Pattern     string            `yaml:"pattern"`      // Regex pattern to match
	Field       string            `yaml:"field"`        // Field to check (e.g., "termination_reason")
	Contains    string            `yaml:"contains"`     // Simple contains check
	Equals      string            `yaml:"equals"`       // Exact match
	StateVars   map[string]string `yaml:"state_vars"`   // State variable conditions
}

// RuleAction defines what action to take
type RuleAction struct {
	Type         string                 `yaml:"type"`         // e.g., "PATCH_RESOURCE", "RESTART", "SCALE"
	Target       string                 `yaml:"target"`       // Target resource
	Patch        map[string]interface{} `yaml:"patch"`        // Patch to apply
	Factor       float64                `yaml:"factor"`       // Multiplier (e.g., 2x for memory)
	Justification []string              `yaml:"justification"`
	Risk         string                 `yaml:"risk"`
}

// DefaultRulesConfig returns common K8s remediation rules
func DefaultRulesConfig() RulesConfig {
	return RulesConfig{
		Enabled: true,
		Rules: []Rule{
			{
				Name:     "oom-killed",
				Priority: 1,
				Condition: RuleCondition{
					Field:  "termination_reason",
					Equals: "OOMKilled",
				},
				Action: RuleAction{
					Type:   "PATCH_RESOURCE",
					Patch:  map[string]interface{}{"resources.limits.memory": "increase"},
					Factor: 2.0,
					Justification: []string{
						"Container terminated due to OOMKilled",
						"Increasing memory limit by 2x",
					},
					Risk: "LOW",
				},
			},
			{
				Name:     "image-pull-backoff",
				Priority: 1,
				Condition: RuleCondition{
					Field:    "status",
					Contains: "ImagePullBackOff",
				},
				Action: RuleAction{
					Type: "DIAGNOSE",
					Justification: []string{
						"Image pull failed",
						"Check image name, tag, and registry credentials",
					},
					Risk: "LOW",
				},
			},
			{
				Name:     "crashloop-backoff",
				Priority: 2,
				Condition: RuleCondition{
					Field:    "status",
					Contains: "CrashLoopBackOff",
				},
				Action: RuleAction{
					Type: "DIAGNOSE",
					Justification: []string{
						"Container is crash-looping",
						"Check application logs and liveness probe configuration",
					},
					Risk: "MEDIUM",
				},
			},
			{
				Name:     "high-restart-count",
				Priority: 3,
				Condition: RuleCondition{
					Pattern: `"restart_count":\s*(\d+)`,
				},
				Action: RuleAction{
					Type: "ALERT",
					Justification: []string{
						"High restart count detected",
						"Investigate root cause before automatic remediation",
					},
					Risk: "MEDIUM",
				},
			},
		},
	}
}

// RulesProvider implements the Provider interface using deterministic rules
type RulesProvider struct {
	*BaseProvider
	config RulesConfig
	rules  []compiledRule
}

type compiledRule struct {
	Rule
	pattern *regexp.Regexp
}

// NewRulesProvider creates a new deterministic rules provider
func NewRulesProvider(config RulesConfig) *RulesProvider {
	p := &RulesProvider{
		BaseProvider: NewBaseProvider("rules", 3, []model.Capability{}), // No LLM capabilities
		config:       config,
		rules:        make([]compiledRule, 0, len(config.Rules)),
	}

	// Compile regex patterns
	for _, rule := range config.Rules {
		cr := compiledRule{Rule: rule}
		if rule.Condition.Pattern != "" {
			cr.pattern, _ = regexp.Compile(rule.Condition.Pattern)
		}
		p.rules = append(p.rules, cr)
	}

	// Sort rules by priority (lower number = higher priority)
	sort.Slice(p.rules, func(i, j int) bool {
		return p.rules[i].Priority < p.rules[j].Priority
	})

	return p
}

// Send attempts to match input against deterministic rules
func (p *RulesProvider) Send(ctx context.Context, req *model.Request) (*model.Response, error) {
	if !p.config.Enabled {
		return nil, NewProviderError(p.Name(), "rules engine disabled", nil)
	}

	start := time.Now()

	// Parse input as JSON to extract fields
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(req.UserPrompt), &input); err != nil {
		// Try to extract from state_variables if present
		input = make(map[string]interface{})
	}

	// Also check the raw input string
	rawInput := req.UserPrompt

	// Try to match rules
	for _, rule := range p.rules {
		if p.matchesCondition(rule, input, rawInput) {
			response := p.buildResponse(rule, input, start)
			return response, nil
		}
	}

	// No rule matched
	return nil, NewProviderError(p.Name(), "no matching rule found", nil)
}

// matchesCondition checks if input matches the rule's condition
func (p *RulesProvider) matchesCondition(rule compiledRule, input map[string]interface{}, rawInput string) bool {
	cond := rule.Condition

	// Track if we have any positive condition to check
	hasCondition := false

	// Check pattern match (if pattern exists, it must match)
	if rule.pattern != nil {
		hasCondition = true
		if !rule.pattern.MatchString(rawInput) {
			return false
		}
	}

	// Check field conditions
	if cond.Field != "" {
		hasCondition = true
		// First try top-level, then try state_variables
		fieldValue := p.getFieldValue(input, cond.Field)
		if fieldValue == "" {
			// Try in state_variables
			if stateVars, ok := input["state_variables"].(map[string]interface{}); ok {
				fieldValue = p.getFieldValue(stateVars, cond.Field)
			}
		}

		if cond.Equals != "" && fieldValue != cond.Equals {
			return false
		}

		if cond.Contains != "" && !strings.Contains(fieldValue, cond.Contains) {
			return false
		}

		// If we have a field condition but the field is empty, don't match
		if fieldValue == "" && (cond.Equals != "" || cond.Contains != "") {
			return false
		}
	}

	// Check state variables (explicit state_variables check)
	if len(cond.StateVars) > 0 {
		hasCondition = true
		stateVars, ok := input["state_variables"].(map[string]interface{})
		if !ok {
			return false
		}

		for key, expected := range cond.StateVars {
			actual := p.getFieldValue(stateVars, key)
			if actual != expected {
				return false
			}
		}
	}

	// Must have at least one condition to match
	return hasCondition
}

// getFieldValue extracts a field value from a nested map
func (p *RulesProvider) getFieldValue(data map[string]interface{}, field string) string {
	parts := strings.Split(field, ".")
	current := data

	for i, part := range parts {
		if val, ok := current[part]; ok {
			if i == len(parts)-1 {
				switch v := val.(type) {
				case string:
					return v
				case float64:
					return strings.TrimRight(strings.TrimRight(
						strings.Replace(string(rune(int(v))), ".", "", -1), "0"), ".")
				default:
					return ""
				}
			}
			if nested, ok := val.(map[string]interface{}); ok {
				current = nested
			} else {
				return ""
			}
		} else {
			return ""
		}
	}
	return ""
}

// buildResponse constructs a response from a matched rule
func (p *RulesProvider) buildResponse(rule compiledRule, input map[string]interface{}, start time.Time) *model.Response {
	action := rule.Action

	// Build target from input if not specified
	target := action.Target
	if target == "" {
		if entities, ok := input["entities"].([]interface{}); ok && len(entities) > 0 {
			if entity, ok := entities[0].(map[string]interface{}); ok {
				if name, ok := entity["name"].(string); ok {
					target = name
				}
			}
		}
	}

	// Build patch with factor applied if needed
	patch := action.Patch
	if action.Factor > 0 && patch != nil {
		patch = p.applyFactor(patch, action.Factor)
	}

	// Build response JSON
	response := map[string]interface{}{
		"action":        action.Type,
		"target":        target,
		"patch":         patch,
		"justification": action.Justification,
		"risk":          action.Risk,
		"matched_rule":  rule.Name,
	}

	content, _ := json.Marshal(response)

	return &model.Response{
		Content:    string(content),
		Provider:   p.Name(),
		Confidence: 1.0, // Deterministic rules have full confidence
		TokensUsed: 0,
		Cached:     false,
		LatencyMs:  time.Since(start).Milliseconds(),
	}
}

// applyFactor multiplies numeric values in the patch by the factor
func (p *RulesProvider) applyFactor(patch map[string]interface{}, factor float64) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range patch {
		if v == "increase" {
			result[k] = map[string]interface{}{
				"operation": "multiply",
				"factor":    factor,
			}
		} else {
			result[k] = v
		}
	}
	return result
}

// IsAvailable always returns true for rules engine
func (p *RulesProvider) IsAvailable() bool {
	return p.config.Enabled
}

// HealthCheck always succeeds for rules engine
func (p *RulesProvider) HealthCheck(ctx context.Context) error {
	return nil
}
