package anthropic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/logger"
)

func createResponseTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func TestTransformResponse_SimpleText(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	openaiResp := map[string]interface{}{
		"id":    "chatcmpl-123",
		"model": "claude-3-5-sonnet-20241022",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello! How can I help you today?",
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(10),
			"completion_tokens": float64(8),
			"total_tokens":      float64(18),
		},
	}

	result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
	require.NoError(t, err)

	anthropicResp, ok := result.(AnthropicResponse)
	require.True(t, ok)

	assert.Equal(t, "message", anthropicResp.Type)
	assert.Equal(t, "assistant", anthropicResp.Role)
	assert.Equal(t, "claude-3-5-sonnet-20241022", anthropicResp.Model)
	assert.Equal(t, "end_turn", anthropicResp.StopReason)

	require.Len(t, anthropicResp.Content, 1)
	assert.Equal(t, "text", anthropicResp.Content[0].Type)
	assert.Equal(t, "Hello! How can I help you today?", anthropicResp.Content[0].Text)

	assert.Equal(t, 10, anthropicResp.Usage.InputTokens)
	assert.Equal(t, 8, anthropicResp.Usage.OutputTokens)
}

func TestTransformResponse_WithToolCalls(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	openaiResp := map[string]interface{}{
		"id":    "chatcmpl-123",
		"model": "claude-3-5-sonnet-20241022",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Let me check that for you.",
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id":   "call_abc123",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "get_weather",
								"arguments": `{"location":"San Francisco","unit":"celsius"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(20),
			"completion_tokens": float64(15),
		},
	}

	result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
	require.NoError(t, err)

	anthropicResp, ok := result.(AnthropicResponse)
	require.True(t, ok)

	assert.Equal(t, "tool_use", anthropicResp.StopReason)
	require.Len(t, anthropicResp.Content, 2)

	assert.Equal(t, "text", anthropicResp.Content[0].Type)
	assert.Equal(t, "Let me check that for you.", anthropicResp.Content[0].Text)

	assert.Equal(t, "tool_use", anthropicResp.Content[1].Type)
	assert.Equal(t, "call_abc123", anthropicResp.Content[1].ID)
	assert.Equal(t, "get_weather", anthropicResp.Content[1].Name)

	require.NotNil(t, anthropicResp.Content[1].Input)
	assert.Equal(t, "San Francisco", anthropicResp.Content[1].Input["location"])
	assert.Equal(t, "celsius", anthropicResp.Content[1].Input["unit"])
}

func TestTransformResponse_MultipleToolCalls(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	openaiResp := map[string]interface{}{
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role": "assistant",
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id":   "call_1",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "get_weather",
								"arguments": `{"location":"SF"}`,
							},
						},
						map[string]interface{}{
							"id":   "call_2",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "get_time",
								"arguments": `{"timezone":"PST"}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{},
	}

	result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
	require.NoError(t, err)

	anthropicResp, ok := result.(AnthropicResponse)
	require.True(t, ok)

	assert.Equal(t, "tool_use", anthropicResp.StopReason)
	require.Len(t, anthropicResp.Content, 2)

	assert.Equal(t, "tool_use", anthropicResp.Content[0].Type)
	assert.Equal(t, "get_weather", anthropicResp.Content[0].Name)

	assert.Equal(t, "tool_use", anthropicResp.Content[1].Type)
	assert.Equal(t, "get_time", anthropicResp.Content[1].Name)
}

func TestTransformResponse_EmptyResponse(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	t.Run("empty_content", func(t *testing.T) {
		// OpenAI response with empty content string
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-empty",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     float64(5),
				"completion_tokens": float64(0),
			},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, "end_turn", anthropicResp.StopReason)
		assert.GreaterOrEqual(t, len(anthropicResp.Content), 0)
	})

	t.Run("null_content", func(t *testing.T) {
		// OpenAI response with null content (tool-only response)
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-null",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": nil,
						"tool_calls": []interface{}{
							map[string]interface{}{
								"id":   "call_only",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "search",
									"arguments": `{"query":"test"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		require.Len(t, anthropicResp.Content, 1)
		assert.Equal(t, "tool_use", anthropicResp.Content[0].Type)
		assert.Equal(t, "search", anthropicResp.Content[0].Name)
	})
}

func TestTransformResponse_MissingUsage(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	t.Run("usage_not_present", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-nousage",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response without usage",
					},
					"finish_reason": "stop",
				},
			},
			// Missing usage field
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, 0, anthropicResp.Usage.InputTokens)
		assert.Equal(t, 0, anthropicResp.Usage.OutputTokens)
	})

	t.Run("partial_usage", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-partial",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response with partial usage",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens": float64(15),
				// Missing completion_tokens
			},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, 15, anthropicResp.Usage.InputTokens)
		assert.Equal(t, 0, anthropicResp.Usage.OutputTokens)
	})
}

func TestTransformResponse_InvalidToolArguments(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	t.Run("malformed_json", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-badjson",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Calling tool",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"id":   "call_bad",
