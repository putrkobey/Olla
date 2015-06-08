package domain

// ProviderMetrics contains metrics extracted from provider responses
// Uses fixed-size types to minimise memory usage in high-frequency scenarios
type ProviderMetrics struct {

	// Provider-specific metadata
	Model        string `json:"model,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
	// Token counts from provider
	InputTokens  int32 `json:"input_tokens,omitempty"`
	OutputTokens int32 `json:"output_tokens,omitempty"`
	TotalTokens  int32 `json:"total_tokens,omitempty"`

	// Timing metrics in milliseconds
	TTFTMs       int32 `json:"ttft_ms,omitempty"`       // Time to first token
	TotalMs      int32 `json:"total_ms,omitempty"`      // Total generation time
	ModelLoadMs  int32 `json:"model_load_ms,omitempty"` // Model loading time (cold starts)
	PromptMs     int32 `json:"prompt_ms,omitempty"`     // Prompt processing time
