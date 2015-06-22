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
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_data",
									"arguments": `{invalid json syntax`,
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

		if err != nil {
			assert.Contains(t, err.Error(), "json", "Error should mention JSON parsing issue")
		} else {
			anthropicResp, ok := result.(AnthropicResponse)
			require.True(t, ok)

			var toolBlock *ContentBlock
			for i := range anthropicResp.Content {
				if anthropicResp.Content[i].Type == "tool_use" {
					toolBlock = &anthropicResp.Content[i]
					break
				}
			}
			require.NotNil(t, toolBlock, "Should have tool_use block")
		}
	})

	t.Run("empty_arguments", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-empty-args",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Calling tool with empty args",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"id":   "call_empty",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "no_params",
									"arguments": `{}`,
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

		require.Len(t, anthropicResp.Content, 2) // text + tool_use
		assert.Equal(t, "tool_use", anthropicResp.Content[1].Type)
		assert.NotNil(t, anthropicResp.Content[1].Input)
		assert.Empty(t, anthropicResp.Content[1].Input) // Empty map
	})
}

func TestTransformResponse_OnlyToolCalls(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	openaiResp := map[string]interface{}{
		"id":    "chatcmpl-toolonly",
		"model": "claude-3-5-sonnet-20241022",
		"choices": []interface{}{
			map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": nil, // No text content
					"tool_calls": []interface{}{
						map[string]interface{}{
							"id":   "call_search",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "web_search",
								"arguments": `{"query":"Anthropic Claude","max_results":5}`,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(25),
			"completion_tokens": float64(10),
		},
	}

	result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
	require.NoError(t, err)

	anthropicResp, ok := result.(AnthropicResponse)
	require.True(t, ok)

	assert.Equal(t, "tool_use", anthropicResp.StopReason)

	require.Len(t, anthropicResp.Content, 1)
	assert.Equal(t, "tool_use", anthropicResp.Content[0].Type)
	assert.Equal(t, "web_search", anthropicResp.Content[0].Name)

	assert.Equal(t, "Anthropic Claude", anthropicResp.Content[0].Input["query"])
	assert.Equal(t, float64(5), anthropicResp.Content[0].Input["max_results"]) // JSON numbers are float64
}

func TestTransformResponse_FinishReasonMapping(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	t.Run("stop_to_end_turn", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-finish",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test response",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, "end_turn", anthropicResp.StopReason, "Normal completion maps to end_turn")
	})

	t.Run("tool_calls_to_tool_use", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-tools",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Calling tool",
						"tool_calls": []interface{}{
							map[string]interface{}{
								"id":   "call_test",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "test_tool",
									"arguments": `{}`,
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

		assert.Equal(t, "tool_use", anthropicResp.StopReason, "Tool invocation maps to tool_use")
	})

	t.Run("length_to_max_tokens", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-length",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test response",
					},
					"finish_reason": "length",
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, "max_tokens", anthropicResp.StopReason, "Token limit maps to max_tokens")
	})

	t.Run("explicit_stop_reason", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-explicit",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Response text",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, "end_turn", anthropicResp.StopReason, "Explicit stop should map to end_turn")
	})

	t.Run("unknown_finish_reason_defaults_to_end_turn", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-unknown",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test",
					},
					"finish_reason": "content_filter", // Unknown reason
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.Equal(t, "end_turn", anthropicResp.StopReason, "Unknown finish_reason should default to end_turn")
	})
}

func TestTransformResponse_MessageIDGeneration(t *testing.T) {
	translator := mustNewTranslator(createResponseTestLogger(), createTestConfig())

	t.Run("generates_anthropic_format_id", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"id":    "chatcmpl-original-123",
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{},
		}

		result, err := translator.TransformResponse(context.Background(), openaiResp, nil)
		require.NoError(t, err)

		anthropicResp, ok := result.(AnthropicResponse)
		require.True(t, ok)

		assert.NotEmpty(t, anthropicResp.ID)
		assert.Contains(t, anthropicResp.ID, "msg_01", "Message ID should start with msg_01 prefix")

		// Verify ID is the expected length (27-29 characters typical for Anthropic)
		// msg_01 (6 chars) + base58 encoded 16 bytes (~22 chars) = ~28 chars total
		assert.GreaterOrEqual(t, len(anthropicResp.ID), 20, "ID should be at least 20 characters")
		assert.LessOrEqual(t, len(anthropicResp.ID), 35, "ID should be at most 35 characters")
	})

	t.Run("generates_unique_ids", func(t *testing.T) {
		openaiResp := map[string]interface{}{
			"model": "claude-3-5-sonnet-20241022",
			"choices": []interface{}{
				map[string]interface{}{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Test",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{},
