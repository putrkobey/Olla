package profile

// VLLMResponse is the response structure from vLLM /v1/models endpoint
type VLLMResponse struct {
	Object string      `json:"object"`
	Data   []VLLMModel `json:"data"`
}

// VLLMModel represents a model in vLLM response with extended metadata
type VLLMModel struct {
	Parent      *string               `json:"parent,omitempty"`        // vLLM-specific: parent model for fine-tuned models
