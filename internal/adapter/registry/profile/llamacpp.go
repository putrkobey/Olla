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
// Reserved for future capability inference enhancements
type LlamaCppMeta struct {
	VocabType int   `json:"vocab_type"`  // Vocabulary type identifier
	NVocab    int   `json:"n_vocab"`     // Vocabulary size
	NCtxTrain int   `json:"n_ctx_train"` // Training context length
	NEmbd     int   `json:"n_embd"`      // Embedding dimensions
	NParams   int64 `json:"n_params"`    // Total parameter count
	Size      int64 `json:"size"`        // Model file size in bytes
}

// LlamaCppProps represents the response from llama.cpp /props endpoint
// Provides server capabilities and configuration - reserved for future phases
type LlamaCppProps struct {
	DefaultGenerationSettings DefaultGenerationSettings `json:"default_generation_settings"`
	TotalSlots                int                       `json:"total_slots"`
}

// DefaultGenerationSettings contains default generation parameters for the server
// Reserved for future capability inference and intelligent defaults
type DefaultGenerationSettings struct {
	Stop             []string `json:"stop"`
	LogitBias        []any    `json:"logit_bias"`
	Samplers         []string `json:"samplers"`
	NPredict         int      `json:"n_predict"`
	Temperature      float64  `json:"temperature"`
	DynaTempRange    float64  `json:"dynatemp_range"`
	DynaTempExponent float64  `json:"dynatemp_exponent"`
	TopK             int      `json:"top_k"`
	TopP             float64  `json:"top_p"`
	MinP             float64  `json:"min_p"`
