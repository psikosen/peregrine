package model

// Request represents an LLM completion request
type Request struct {
	// AgentRole identifies the calling agent ("modeler" or "solver")
	AgentRole string `json:"agent_role"`

	// SystemPrompt is the static instruction set (cached)
	SystemPrompt string `json:"system_prompt"`

	// UserPrompt is the dynamic content (suffix tokens)
	UserPrompt string `json:"user_prompt"`

	// Temperature controls randomness (0.0 - 1.0)
	Temperature float64 `json:"temperature"`

	// MaxTokens limits the response length
	MaxTokens int `json:"max_tokens"`

	// Metadata contains additional routing hints
	Metadata map[string]string `json:"metadata"`

	// RequiredCapabilities specifies what the provider must support
	RequiredCapabilities []Capability `json:"required_capabilities"`
}

// Response represents an LLM completion response
type Response struct {
	// Content is the generated text
	Content string `json:"content"`

	// Provider identifies which backend handled this request
	Provider string `json:"provider"`

	// Confidence is the model's self-reported confidence (0.0 - 1.0)
	Confidence float64 `json:"confidence"`

	// TokensUsed is the total token count
	TokensUsed int `json:"tokens_used"`

	// Cached indicates if the prompt was served from cache
	Cached bool `json:"cached"`

	// LatencyMs is the processing time in milliseconds
	LatencyMs int64 `json:"latency_ms"`
}

// UnsureResponse creates a response indicating the model is unsure
func UnsureResponse(provider, reason string) *Response {
	return &Response{
		Content:    `{"status": "UNSURE", "reason": "` + reason + `"}`,
		Provider:   provider,
		Confidence: 0.0,
		TokensUsed: 0,
		Cached:     false,
	}
}
