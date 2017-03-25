package translator

import (
	"context"
	"io"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// converts between api formats (e.g., anthropic → openai)
// lets olla accept multiple formats while using openai internally
type RequestTranslator interface {
	// converts incoming request to openai format
	// returns transformed request with metadata for response translation
	TransformRequest(ctx context.Context, r *http.Request) (*TransformedRequest, error)

	// converts openai response back to original format
	// uses original request to keep context (model, metadata)
	TransformResponse(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error)

	// handles streaming response conversion
	// reads openai sse stream and writes target format
	TransformStreamingResponse(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error

	// returns translator name (eg "anthropic")
	Name() string
}

// holds converted request and metadata
// preserving info for response translation
type TransformedRequest struct {
	OpenAIRequest map[string]interface{} // Converted OpenAI format request body
	Metadata      map[string]interface{} // Additional context for response translation
	ModelName     string                 // Extracted model name for routing
	TargetPath    string                 // Target API path for the backend (e.g., "/v1/chat/completions" - proxy handles /olla prefix)
	OriginalBody  []byte                 // Original request body for response translation context
	IsStreaming   bool                   // Whether response should stream
}

// optional interface for translators to define their api endpoints
// if not implemented, routes need manual registration
type PathProvider interface {
	GetAPIPath() string // Returns the API path (e.g., "/olla/anthropic/v1/messages")
}

// optional interface for translators to declare the backend path their translated
// request is sent to (e.g. "/v1/chat/completions"). Endpoint selection uses it to
// ask which backends serve that path, rather than matching the translator's own
// public route (/olla/anthropic/v1/messages) against backend profile paths.
type TargetPathProvider interface {
	TranslationTargetPath() string
}

// optional interface for custom error formatting per translator (eg anthropic error structure)
// falls back to generic json if not implemented
type ErrorWriter interface {
	WriteError(w http.ResponseWriter, err error, statusCode int)
}

// optional interface for translators whose WriteError logs; lets callers that already report
// a rejection through a sampled log write the same response without a second line per request
type QuietErrorWriter interface {
	WriteErrorQuiet(w http.ResponseWriter, err error, statusCode int)
}

// optional interface for translators that support token counting
// enables api compatibility with token estimation endpoints
type TokenCounter interface {
	CountTokens(ctx context.Context, r *http.Request) (*TokenCountResponse, error)
}

// optional interface for translators that need to control the wire serialisation
// of their token count response. When implemented, the handler uses SerialiseCountTokens
// instead of encoding TokenCountResponse directly, allowing translators to emit only
// the fields their spec defines (e.g. Anthropic: {"input_tokens":N} only).
type TokenCountSerializer interface {
	SerialiseCountTokens(resp *TokenCountResponse) ([]byte, error)
}

// optional interface for translators that can cheaply estimate input token count
// from a pre-buffered original request body. Used to seed input_tokens in streaming
// message_start events before the upstream usage chunk arrives.
type InputTokenEstimator interface {
	EstimateInputTokens(originalBodyBytes []byte) int
}

// represents token count result
type TokenCountResponse struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// optional interface for translators that provide model listings
// exposes available models in their native api format
