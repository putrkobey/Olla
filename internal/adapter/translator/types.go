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
