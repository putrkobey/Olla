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

