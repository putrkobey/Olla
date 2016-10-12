package profile

// OpenAICompatibleResponse is the response structure from OpenAI-compatible /v1/models endpoint
type OpenAICompatibleResponse struct {
	Object string                  `json:"object"`
	Data   []OpenAICompatibleModel `json:"data"`
