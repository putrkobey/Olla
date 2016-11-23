package profile

// VLLMMLXResponse is the response structure from vLLM-MLX /v1/models endpoint.
// vLLM-MLX uses the standard OpenAI-compatible list format.
type VLLMMLXResponse struct {
	Object string         `json:"object"`
	Data   []VLLMMLXModel `json:"data"`
}
