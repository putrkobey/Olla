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
				return nil, fmt.Errorf("failed to convert tool_choice: %w", tcErr)
			}
			openaiReq["tool_choice"] = openaiToolChoice
		}
	}

	t.logger.Debug("Transformed Anthropic request to OpenAI",
		"model", anthropicReq.Model,
		"message_count", len(anthropicReq.Messages),
		"has_tools", len(anthropicReq.Tools) > 0,
		"streaming", anthropicReq.Stream)

	return &translator.TransformedRequest{
		OpenAIRequest: openaiReq,
		OriginalBody:  body,
		ModelName:     anthropicReq.Model,
		IsStreaming:   anthropicReq.Stream,
		TargetPath:    openAIChatCompletionsPath,
		Metadata: map[string]interface{}{
			"format": "anthropic",
		},
	}, nil
}

// convert messages + inject system prompt if present
func (t *Translator) convertMessages(anthropicMessages []AnthropicMessage, systemPrompt interface{}) ([]map[string]interface{}, error) {
	openaiMessages := make([]map[string]interface{}, 0, len(anthropicMessages)+1)

	// openai wants system as first message, can be string or content blocks
	if systemPrompt != nil {
		systemContent := t.convertSystemPrompt(systemPrompt)
		if systemContent != nil {
			openaiMessages = append(openaiMessages, map[string]interface{}{
				"role":    "system",
				"content": systemContent,
			})
		}
	}

	for _, msg := range anthropicMessages {
		converted, err := t.convertSingleMessage(msg)
		if err != nil {
			return nil, err
		}
		openaiMessages = append(openaiMessages, converted...)
	}

	return openaiMessages, nil
}

// convert single message, might produce multiple openai messages (eg user + tool results)
func (t *Translator) convertSingleMessage(msg AnthropicMessage) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, 2)

	if contentStr, ok := msg.Content.(string); ok {
		if contentStr != "" {
			result = append(result, map[string]interface{}{
				"role":    msg.Role,
				"content": contentStr,
			})
		}
		return result, nil
	}

	// anthropic uses content blocks for rich messages
	contentBlocks, ok := msg.Content.([]interface{})
	if !ok {
		if contentMap, ok := msg.Content.(map[string]interface{}); ok {
			contentBlocks = []interface{}{contentMap}
		} else {
			return nil, fmt.Errorf("invalid content type: %T", msg.Content)
		}
	}

	// user msgs can have text + tool results, assistant msgs have text + tool uses
	if msg.Role == "user" {
		userMsg, toolMsgs := t.convertUserMessage(contentBlocks)
		// tool messages must immediately follow the assistant tool_calls message;
		// OpenAI-compatible backends reject any other role between them.
		result = append(result, toolMsgs...)
		if userMsg != nil {
			result = append(result, userMsg)
		}
