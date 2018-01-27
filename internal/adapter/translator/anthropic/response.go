package anthropic

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"

	"github.com/putrkobey/Olla/internal/util"
)

// convert openai response to anthropic format
// maps choices/messages to content blocks, handles tools and token usage
func (t *Translator) TransformResponse(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error) {
	// Parse OpenAI response
	respMap, ok := openaiResp.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid OpenAI response type: %T", openaiResp)
	}

	// Extract choices
	choices, ok := respMap["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, errors.New("no choices in OpenAI response")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid choice format")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		return nil, errors.New("no message in choice")
	}

	// grab finish_reason for stop_reason mapping
	finishReason := ""
	if fr, ok := choice["finish_reason"].(string); ok {
		finishReason = fr
	}

	// Build Anthropic response
	anthropicResp := AnthropicResponse{
		ID:    t.generateMessageID(),
		Type:  "message",
		Role:  "assistant",
		Model: t.extractModel(respMap),
	}

	// convert content and figure out stop_reason
	content, stopReason := t.convertResponseContent(message, finishReason)
	anthropicResp.Content = content
	anthropicResp.StopReason = stopReason
	anthropicResp.StopSequence = nil

	// Convert usage
	if usage, ok := respMap["usage"].(map[string]interface{}); ok {
		anthropicResp.Usage = t.convertUsage(usage)
	}

	t.logger.Debug("Transformed OpenAI response to Anthropic",
		"content_blocks", len(content),
		"stop_reason", stopReason,
		"input_tokens", anthropicResp.Usage.InputTokens,
		"output_tokens", anthropicResp.Usage.OutputTokens)

	// Log response to inspector if enabled
	if t.inspector.Enabled() {
		sessionID := t.getSessionID(original)
		if respBytes, err := json.Marshal(anthropicResp); err == nil {
			if err := t.inspector.LogResponse(sessionID, respBytes); err != nil {
				t.logger.Warn("Failed to log response to inspector", "error", err)
			}
		}
	}

	return anthropicResp, nil
}

// parse text and tool_calls from openai message into content blocks
// finish_reason determines what stop_reason to use
func (t *Translator) convertResponseContent(message map[string]interface{}, finishReason string) ([]ContentBlock, string) {
	var content []ContentBlock

	// Prepend a thinking block when the backend includes chain-of-thought reasoning.
	// "reasoning" is used by Ollama/LM Studio/Lemonade; "reasoning_content" by vLLM/SGLang/DeepSeek.
	// Local backends do not produce a cryptographic signature, so we omit the signature field.
	if reasoning := extractNonStreamingReasoning(message); reasoning != "" {
		content = append(content, ContentBlock{
			Type:     contentTypeThinking,
			Thinking: reasoning,
		})
	}

	// Handle text content
	if textContent, ok := message["content"].(string); ok && textContent != "" {
		content = append(content, ContentBlock{
			Type: contentTypeText,
			Text: textContent,
		})
	}

	// Handle tool calls
	if toolCalls, ok := message["tool_calls"].([]interface{}); ok && len(toolCalls) > 0 {
		for _, tc := range toolCalls {
			toolCall, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}

			toolUse := t.convertToToolUse(toolCall)
			if toolUse != nil {
				content = append(content, *toolUse)
			}
		}
	}

	// anthropic needs at least one block, even if empty
	if len(content) == 0 {
		content = append(content, ContentBlock{
			Type: contentTypeText,
			Text: "",
		})
	}

	stopReason := mapFinishReasonToStopReason(finishReason)

	return content, stopReason
}

// map openai finish_reason to anthropic stop_reason
// shared between normal and streaming paths
