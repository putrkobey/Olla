package profile

// OllamaResponse is the response structure from Ollama's /api/tags endpoint
type OllamaResponse struct {
	Models []OllamaModel `json:"models"`
}

// OllamaModel represents a model in Ollama's response
type OllamaModel struct {
