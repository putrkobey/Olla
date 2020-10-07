package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/putrkobey/Olla/internal/adapter/translator"
)

// convert anthropic format to openai, handles messages/tools/params
func (t *Translator) TransformRequest(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
	// limit request body to prevent DOS, uses configured max size
	limitedBody := io.LimitReader(r.Body, t.maxMessageSize)
	defer r.Body.Close()

	// Parse leniently: as a proxy we must not be stricter than the upstream API we emulate.
	// Unknown or newer Anthropic fields (e.g. context_management, experimental beta fields)
	// are silently ignored rather than rejected. The streaming decoder is kept for memory
	// efficiency on large requests.
	var anthropicReq AnthropicRequest
	decoder := json.NewDecoder(limitedBody)

	if err := decoder.Decode(&anthropicReq); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic request: %w", err)
	}

	if err := anthropicReq.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// re-marshal to get body bytes for passthrough
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log request to inspector if enabled
	if t.inspector.Enabled() {
		sessionID := t.getSessionID(r)
		if lerr := t.inspector.LogRequest(sessionID, anthropicReq.Model, body); lerr != nil {
			t.logger.Warn("Failed to log request to inspector", "error", lerr)
		}
	}

	// Convert to OpenAI format
	openaiReq := make(map[string]interface{})

	// Basic parameters - direct mapping
	openaiReq["model"] = anthropicReq.Model
	openaiReq["max_tokens"] = anthropicReq.MaxTokens
	openaiReq["stream"] = anthropicReq.Stream

	// Optional parameters
	if anthropicReq.Temperature != nil {
		openaiReq["temperature"] = *anthropicReq.Temperature
	}
	if anthropicReq.TopP != nil {
		openaiReq["top_p"] = *anthropicReq.TopP
	}
	if len(anthropicReq.StopSequences) > 0 {
		openaiReq["stop"] = anthropicReq.StopSequences
	}

	// Convert messages (includes system prompt injection)
	openaiMessages, err := t.convertMessages(anthropicReq.Messages, anthropicReq.System)
	if err != nil {
		return nil, fmt.Errorf("failed to convert messages: %w", err)
	}
	openaiReq["messages"] = openaiMessages

	// Convert tools if present
	if len(anthropicReq.Tools) > 0 {
		openaiTools := t.convertTools(anthropicReq.Tools)
		openaiReq["tools"] = openaiTools

		// Convert tool_choice
		if anthropicReq.ToolChoice != nil {
			openaiToolChoice, tcErr := t.convertToolChoice(anthropicReq.ToolChoice)
			if tcErr != nil {
