package profile

// VLLMResponse is the response structure from vLLM /v1/models endpoint
type VLLMResponse struct {
	Object string      `json:"object"`
	Data   []VLLMModel `json:"data"`
}

// VLLMModel represents a model in vLLM response with extended metadata
type VLLMModel struct {
	Parent      *string               `json:"parent,omitempty"`        // vLLM-specific: parent model for fine-tuned models
	MaxModelLen *int64                `json:"max_model_len,omitempty"` // vLLM-specific: maximum context length
	ID          string                `json:"id"`
	Object      string                `json:"object"`
	OwnedBy     string                `json:"owned_by"`
	Root        string                `json:"root,omitempty"`       // vLLM-specific: root model identifier
	Permission  []VLLMModelPermission `json:"permission,omitempty"` // vLLM-specific: access permissions
	Created     int64                 `json:"created"`
}

// VLLMModelPermission represents vLLM's granular permission system
type VLLMModelPermission struct {
	Group              *string `json:"group"`
