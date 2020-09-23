package profile

// LlamaCppResponse represents the response structure from llama.cpp /v1/models endpoint
// Reference: https://github.com/ggerganov/llama.cpp/blob/master/examples/server/README.md
//
// llama.cpp returns dual format: Ollama-compatible 'models' array and OpenAI-compatible 'data' array
// This dual format provides compatibility with both API standards, though content is identical
// Note: llama.cpp typically serves a single model per instance, so both arrays usually have one element
type LlamaCppResponse struct {
	Object string          `json:"object"` // Always "list"
	Data   []LlamaCppModel `json:"data"`   // OpenAI-compatible format (use this)
	Models []LlamaCppModel `json:"models"` // Ollama-compatible format (optional duplicate)
}

// LlamaCppModel represents a model in llama.cpp response
// llama.cpp uses OpenAI-compatible format with optional extended metadata
type LlamaCppModel struct {

	// llama.cpp extended metadata
	Meta *LlamaCppMeta `json:"meta,omitempty"`
	// Standard OpenAI-compatible fields
	ID      string `json:"id"`       // Model identifier (often just model filename)
	Object  string `json:"object"`   // Always "model"
	OwnedBy string `json:"owned_by"` // Typically "llamacpp" or organisation from model path
	Created int64  `json:"created"`  // Unix timestamp (server start time or model load time)

}

// LlamaCppMeta represents extended model metadata from llama.cpp
// Provides detailed information about model architecture and capabilities
