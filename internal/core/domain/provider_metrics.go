package domain

// ProviderMetrics contains metrics extracted from provider responses
// Uses fixed-size types to minimise memory usage in high-frequency scenarios
type ProviderMetrics struct {

	// Provider-specific metadata
	Model        string `json:"model,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
