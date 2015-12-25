package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransformRequest_WithThinkingField tests that the thinking field is accepted
// This verifies the Thinking field is a known field and won't cause "unknown field" errors
func TestTransformRequest_WithThinkingField(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	tests := []struct {
		name     string
		thinking interface{}
		desc     string
	}{
		{
			name:     "thinking_as_object",
			thinking: map[string]interface{}{"type": "enabled", "budget_tokens": 1000},
			desc:     "Thinking field as object with configuration",
		},
		{
			name:     "thinking_as_string",
			thinking: "enabled",
			desc:     "Thinking field as string",
		},
		{
			name:     "thinking_as_boolean",
			thinking: true,
			desc:     "Thinking field as boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anthropicReq := AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{
						Role:    "user",
						Content: "Solve this complex problem step by step",
					},
				},
				Thinking: tt.thinking,
			}

			body, err := json.Marshal(anthropicReq)
			require.NoError(t, err)

			// Verify the JSON contains the thinking field
			var parsedBody map[string]interface{}
			err = json.Unmarshal(body, &parsedBody)
			require.NoError(t, err)
			assert.Contains(t, parsedBody, "thinking", tt.desc)

			req := &http.Request{
				Body: io.NopCloser(bytes.NewReader(body)),
			}

			// Transform should succeed without "unknown field" error
			result, err := translator.TransformRequest(context.Background(), req)
			require.NoError(t, err, tt.desc)
			assert.NotNil(t, result)

			// Thinking field is Anthropic-specific and not passed to OpenAI format
			// It's used for extended thinking mode which is transparent to translation
			_, hasThinking := result.OpenAIRequest["thinking"]
			assert.False(t, hasThinking, "Thinking field should not be passed to OpenAI format")
		})
	}
}

// TestTransformRequest_SystemPromptAsArray tests system prompt with content blocks
// Anthropic API supports system prompts as arrays of content blocks
func TestTransformRequest_SystemPromptAsArray(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		System: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "You are a helpful assistant. ",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Always be concise.",
			},
		},
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

	req := &http.Request{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	result, err := translator.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	messages, ok := result.OpenAIRequest["messages"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, messages, 2) // system + user message

	// First message should be system with concatenated text
	assert.Equal(t, "system", messages[0]["role"])
	assert.Equal(t, "You are a helpful assistant. Always be concise.", messages[0]["content"])

	// Second message should be user
	assert.Equal(t, "user", messages[1]["role"])
	assert.Equal(t, "Hello", messages[1]["content"])
}

// TestTransformRequest_SystemPromptArrayWithEmptyBlocks tests handling of empty content blocks
func TestTransformRequest_SystemPromptArrayWithEmptyBlocks(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		System: []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": "",
			},
			map[string]interface{}{
				"type": "text",
				"text": "Only this matters",
			},
		},
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

	req := &http.Request{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	result, err := translator.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	messages, ok := result.OpenAIRequest["messages"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, messages, 2)

	// System message should contain only non-empty text
	assert.Equal(t, "system", messages[0]["role"])
	assert.Equal(t, "Only this matters", messages[0]["content"])
}

// TestTransformRequest_SystemPromptArrayEmpty tests empty system prompt array
func TestTransformRequest_SystemPromptArrayEmpty(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		System:    []interface{}{},
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello",
			},
		},
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

	req := &http.Request{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	result, err := translator.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	messages, ok := result.OpenAIRequest["messages"].([]map[string]interface{})
	require.True(t, ok)
	// Should only have user message, no system message for empty array
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0]["role"])
}

// TestTransformRequest_SystemPromptString tests traditional string system prompt still works
func TestTransformRequest_SystemPromptString(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		System:    "You are a helpful assistant",
		Messages: []AnthropicMessage{
