package profile

// LMDeployResponse is the response structure from LMDeploy /v1/models endpoint.
// The shape follows the OpenAI ModelList format but with LMDeploy-specific field
// values — notably owned_by defaults to "lmdeploy" and there is no max_model_len.
type LMDeployResponse struct {
	Object string          `json:"object"`
	Data   []LMDeployModel `json:"data"`
}
