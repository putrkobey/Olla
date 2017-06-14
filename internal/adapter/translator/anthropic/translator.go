package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/pkg/pool"
)

// openAIChatCompletionsPath is where translated requests are sent. Backends
// are selected on this path, not on the Anthropic route the client called.
const openAIChatCompletionsPath = "/v1/chat/completions"

// Translator converts between Anthropic and OpenAI API formats
// Uses buffer pooling to minimise memory allocations during translation
type Translator struct {
	logger         logger.StyledLogger
	bufferPool     *pool.Pool[*bytes.Buffer]
	inspector      *inspector.Simple
	config         config.AnthropicTranslatorConfig
	maxMessageSize int64 // derived from config
}

// NewTranslator creates a new Anthropic translator instance.
// Uses a buffer pool to reduce GC pressure during high-throughput operations.
// Accepts configuration for request size limits and streaming behaviour.
// Returns an error if the buffer pool cannot be constructed (unreachable in
// production because bytes.NewBuffer never returns nil, but exposed here to
// honour the no-panic policy).
func NewTranslator(log logger.StyledLogger, cfg config.AnthropicTranslatorConfig) (*Translator, error) {
	// Create buffer pool with 4KB initial capacity.
	// This size fits most chat completions without reallocation.
	bufferPool, err := pool.NewLitePool(func() *bytes.Buffer {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	})
	if err != nil {
		return nil, fmt.Errorf("translator: failed to initialise buffer pool: %w", err)
	}

	// Apply defaults if needed
	maxSize := cfg.MaxMessageSize
	if maxSize <= 0 {
		maxSize = 10 << 20 // 10MB default
		log.Warn("Invalid or missing max_message_size, using default", "default", maxSize)
	}

	// Create inspector for debugging
	insp := inspector.NewSimple(
		cfg.Inspector.Enabled,
		cfg.Inspector.OutputDir,
		cfg.Inspector.SessionHeader,
		log,
	)

	return &Translator{
		logger:         log,
		bufferPool:     bufferPool,
		config:         cfg,
		maxMessageSize: maxSize,
		inspector:      insp,
	}, nil
}

// Name returns the translator identifier
func (t *Translator) Name() string {
	return "anthropic"
}

// GetAPIPath implements PathProvider interface for dynamic route registration
// Returns the Anthropic Messages API endpoint path
func (t *Translator) GetAPIPath() string {
	return "/olla/anthropic/v1/messages"
}

// TranslationTargetPath implements translator.TargetPathProvider: the OpenAI
// path an Anthropic request is translated onto when it cannot be passed through.
func (t *Translator) TranslationTargetPath() string {
	return openAIChatCompletionsPath
}

// MaxBodySize implements BodySizeLimiter so the handler can apply the
// translator's configured limit when reading the request body, rather
// than hardcoding a value.
func (t *Translator) MaxBodySize() int64 {
	return t.maxMessageSize
}

// getSessionID extracts the session ID from the request using a fallback chain.
// this ensures we always have a valid session id for request tracking:
// 1. try the configured session header (for custom session management)
// 2. if there's none, fall back to X-Request-ID (standard request correlation)
// 3. if that's still no go we, use default constant
// the chain exists because different callers may use different correlation mechanisms.
func (t *Translator) getSessionID(r *http.Request) string {
	const HeaderRequestId = "X-Request-ID"

	sessionID := r.Header.Get(t.inspector.GetSessionHeader())
	if sessionID == "" {
		sessionID = r.Header.Get(HeaderRequestId)
		if sessionID == "" {
			sessionID = defaultSessionID
		}
	}

	return sessionID
}

// forEachSystemContentBlock iterates over system prompt content blocks regardless of input format.
// This is a convenience wrapper around the standalone forEachSystemBlock function.
// See forEachSystemBlock in token_count.go for the implementation details.
func (t *Translator) forEachSystemContentBlock(system interface{}, fn func(block ContentBlock) error) error {
	return forEachSystemBlock(system, fn)
}

// WriteError implements ErrorWriter interface for Anthropic-specific error formatting
// Formats errors according to Anthropic's error schema
// See: https://docs.anthropic.com/claude/reference/errors
func (t *Translator) WriteError(w http.ResponseWriter, err error, statusCode int) {
	t.logger.Error("Anthropic request failed",
		"error", err.Error(),
		"status", statusCode)
	t.WriteErrorQuiet(w, err, statusCode)
}

// WriteErrorQuiet writes the same error response as WriteError without the ERROR log line,
// for rejections the caller already reports through a sampled log.
func (t *Translator) WriteErrorQuiet(w http.ResponseWriter, err error, statusCode int) {
	// Map HTTP status codes to Anthropic error types.
	// overloaded_error is reserved for Anthropic's own infrastructure being overloaded;
	// a generic backend 503 is an api_error per litellm and bifrost conventions.
	// timeout_error maps to 504/408 as defined by litellm and the Anthropic error taxonomy.
	errorType := "api_error"
	switch statusCode {
	case http.StatusBadRequest:
		errorType = "invalid_request_error"
	case http.StatusUnauthorized:
		errorType = "authentication_error"
	case http.StatusForbidden:
		errorType = "permission_error"
	case http.StatusNotFound:
		errorType = "not_found_error"
	case http.StatusRequestEntityTooLarge:
		// Anthropic error taxonomy: oversized request body maps to request_too_large.
		errorType = "request_too_large"
	case http.StatusTooManyRequests:
		errorType = "rate_limit_error"
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		errorType = "timeout_error"
	}

	// Anthropic error format
	errorResp := map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    errorType,
			"message": err.Error(),
