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
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	MessageFormat     = "anthropic"
	ClaudeSonnetModel = "claude-sonnet-4-20250929"
	ClaudeOpusModel   = "claude-opus-4-1-20250805"
	ClaudeHaikuModel  = "claude-haiku-4-5-20251001"
)

// helper for tests to setup logger
func createIntegrationTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

// test config helper
func createTestConfig() config.AnthropicTranslatorConfig {
	return config.AnthropicTranslatorConfig{
		Enabled:        true,
		MaxMessageSize: 10 << 20, // 10MB
	}
}

// mustNewTranslator wraps NewTranslator for test use. NewTranslator now returns
// an error so the no-panic policy is satisfied in production code; tests use
// this helper to keep call-sites terse and fatal on unexpected failures.
func mustNewTranslator(log logger.StyledLogger, cfg config.AnthropicTranslatorConfig) *Translator {
	tr, err := NewTranslator(log, cfg)
	if err != nil {
		panic("mustNewTranslator: " + err.Error())
	}
	return tr
}

// setup test http request
func createHTTPRequest(t *testing.T, anthropicReq AnthropicRequest) *http.Request {
	t.Helper()

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err, "Failed to marshal Anthropic request")

	req, err := http.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	require.NoError(t, err, "Failed to create HTTP request")

	return req
}

// mock backend response
func simulateBackendResponse(content string, toolCalls []map[string]interface{}, finishReason string) map[string]interface{} {
	message := map[string]interface{}{
		"role": "assistant",
	}

	// set content - null if no text, only tool calls
	if content != "" {
		message["content"] = content
	} else if len(toolCalls) > 0 {
		message["content"] = nil
	}

	// convert tool calls to required interface format
	if len(toolCalls) > 0 {
		converted := make([]interface{}, len(toolCalls))
		for i, tc := range toolCalls {
			converted[i] = tc
		}
		message["tool_calls"] = converted
	}

	return map[string]interface{}{
		"id":    "chatcmpl-test-123",
		"model": ClaudeSonnetModel,
		"choices": []interface{}{
			map[string]interface{}{
				"message":       message,
				"finish_reason": finishReason,
				"index":         0,
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(25),
			"completion_tokens": float64(15),
			"total_tokens":      float64(40),
		},
	}
}

func TestAnthropicToOpenAIToAnthropic_RoundTrip(t *testing.T) {
	translator := mustNewTranslator(createIntegrationTestLogger(), createTestConfig())
	ctx := context.Background()

	t.Run("simple_text_conversation", func(t *testing.T) {
		// setup test request
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{
					Role:    "user",
					Content: "Hello, how are you?",
				},
			},
		}

		// transform to openai for backend
		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err, "Request transformation should succeed")

		// verify backend request format
		assert.Equal(t, ClaudeSonnetModel, transformed.ModelName)
		assert.False(t, transformed.IsStreaming)
		assert.Equal(t, MessageFormat, transformed.Metadata["format"])

		openaiReq := transformed.OpenAIRequest
		assert.Equal(t, ClaudeSonnetModel, openaiReq["model"])
		assert.Equal(t, 1024, openaiReq["max_tokens"])
		assert.Equal(t, false, openaiReq["stream"])

		messages, ok := openaiReq["messages"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, messages, 1)
		assert.Equal(t, "user", messages[0]["role"])
		assert.Equal(t, "Hello, how are you?", messages[0]["content"])

		// 4. Simulate backend response
		backendResp := simulateBackendResponse(
			"I'm doing well, thank you for asking! How can I help you today?",
			nil,
			"stop",
		)

		// 5. Transform response back to Anthropic format
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err, "Response transformation should succeed")

		// 6. Verify round-trip preservation
		resp, ok := anthropicResp.(AnthropicResponse)
		require.True(t, ok, "Response should be AnthropicResponse type")

		assert.Equal(t, "message", resp.Type)
		assert.Equal(t, "assistant", resp.Role)
		assert.Equal(t, ClaudeSonnetModel, resp.Model)
		assert.Equal(t, "end_turn", resp.StopReason)
		assert.NotEmpty(t, resp.ID)

		require.Len(t, resp.Content, 1)
		assert.Equal(t, "text", resp.Content[0].Type)
		assert.Equal(t, "I'm doing well, thank you for asking! How can I help you today?", resp.Content[0].Text)

		assert.Equal(t, 25, resp.Usage.InputTokens)
		assert.Equal(t, 15, resp.Usage.OutputTokens)
	})

	t.Run("multi_turn_conversation", func(t *testing.T) {
		// Test multi-turn conversation with history
		anthropicReq := AnthropicRequest{
			Model:     ClaudeOpusModel,
			MaxTokens: 2048,
			Messages: []AnthropicMessage{
				{
					Role:    "user",
					Content: "What is the capital of France?",
				},
				{
					Role:    "assistant",
					Content: "The capital of France is Paris.",
				},
				{
					Role:    "user",
					Content: "What about Germany?",
				},
			},
		}

		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		// Verify conversation history preserved
		messages := transformed.OpenAIRequest["messages"].([]map[string]interface{})
		require.Len(t, messages, 3)
		assert.Equal(t, "user", messages[0]["role"])
		assert.Equal(t, "assistant", messages[1]["role"])
		assert.Equal(t, "user", messages[2]["role"])
		assert.Equal(t, "What about Germany?", messages[2]["content"])

		// Simulate response with correct model
		backendResp := simulateBackendResponse("The capital of Germany is Berlin.", nil, "stop")
		backendResp["model"] = ClaudeOpusModel
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		resp := anthropicResp.(AnthropicResponse)
		assert.Equal(t, ClaudeOpusModel, resp.Model)
		assert.Equal(t, "The capital of Germany is Berlin.", resp.Content[0].Text)
	})

	t.Run("with_system_prompt", func(t *testing.T) {
		// Test system prompt injection
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			System:    "You are a helpful assistant that speaks like a pirate.",
			Messages: []AnthropicMessage{
				{
					Role:    "user",
					Content: "Hello",
				},
			},
		}

		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		// Verify system prompt injected as first message
		messages := transformed.OpenAIRequest["messages"].([]map[string]interface{})
		require.Len(t, messages, 2)
		assert.Equal(t, "system", messages[0]["role"])
		assert.Equal(t, "You are a helpful assistant that speaks like a pirate.", messages[0]["content"])
		assert.Equal(t, "user", messages[1]["role"])

		// Simulate response
		backendResp := simulateBackendResponse("Ahoy there, matey! How can I help ye today?", nil, "stop")
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		resp := anthropicResp.(AnthropicResponse)
		assert.Equal(t, "Ahoy there, matey! How can I help ye today?", resp.Content[0].Text)
	})

	t.Run("with_optional_parameters", func(t *testing.T) {
		// Test parameter mapping
		temp := 0.7
		topP := 0.9

		anthropicReq := AnthropicRequest{
			Model:         ClaudeSonnetModel,
			MaxTokens:     2048,
			Temperature:   &temp,
			TopP:          &topP,
			StopSequences: []string{"END", "STOP"},
			Messages: []AnthropicMessage{
				{
					Role:    "user",
					Content: "Count to 5",
				},
			},
		}

		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		// Verify parameters mapped correctly
		assert.Equal(t, 2048, transformed.OpenAIRequest["max_tokens"])
		assert.Equal(t, 0.7, transformed.OpenAIRequest["temperature"])
		assert.Equal(t, 0.9, transformed.OpenAIRequest["top_p"])
		assert.Equal(t, []string{"END", "STOP"}, transformed.OpenAIRequest["stop"])

		// Simulate response
		backendResp := simulateBackendResponse("1, 2, 3, 4, 5", nil, "stop")
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		resp := anthropicResp.(AnthropicResponse)
		assert.Equal(t, "1, 2, 3, 4, 5", resp.Content[0].Text)
	})

	t.Run("finish_reason_mapping", func(t *testing.T) {
		// Test all finish_reason mappings
		testCases := []struct {
			name               string
			finishReason       string
			expectedStopReason string
		}{
			{"stop_to_end_turn", "stop", "end_turn"},
			{"length_to_max_tokens", "length", "max_tokens"},
			{"unknown_defaults_to_end_turn", "content_filter", "end_turn"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				anthropicReq := AnthropicRequest{
					Model:     ClaudeSonnetModel,
					MaxTokens: 10,
					Messages: []AnthropicMessage{
						{Role: "user", Content: "Hi"},
					},
				}

				httpReq := createHTTPRequest(t, anthropicReq)
				_, err := translator.TransformRequest(ctx, httpReq)
				require.NoError(t, err)

				backendResp := simulateBackendResponse("Response", nil, tc.finishReason)
				anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
				require.NoError(t, err)

				resp := anthropicResp.(AnthropicResponse)
				assert.Equal(t, tc.expectedStopReason, resp.StopReason,
					"finish_reason %s should map to stop_reason %s", tc.finishReason, tc.expectedStopReason)
			})
		}
	})

	t.Run("message_id_generation", func(t *testing.T) {
		// Test that message IDs are generated
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			Messages:  []AnthropicMessage{{Role: "user", Content: "Test"}},
		}

		httpReq := createHTTPRequest(t, anthropicReq)
		_, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		backendResp := simulateBackendResponse("Response", nil, "stop")
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		resp := anthropicResp.(AnthropicResponse)
		assert.NotEmpty(t, resp.ID)
		assert.Contains(t, resp.ID, "msg_", "Message ID should start with msg_ prefix")
	})

	t.Run("complex_content_blocks", func(t *testing.T) {
		// Test content with multiple text blocks (should concatenate)
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type": "text",
							"text": "First part ",
						},
						map[string]interface{}{
							"type": "text",
							"text": "second part",
						},
					},
				},
			},
		}

		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		// Verify text blocks concatenated
		messages := transformed.OpenAIRequest["messages"].([]map[string]interface{})
		require.Len(t, messages, 1)
		assert.Equal(t, "First part second part", messages[0]["content"])

		// Round-trip response
		backendResp := simulateBackendResponse("Combined response", nil, "stop")
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		resp := anthropicResp.(AnthropicResponse)
		assert.Equal(t, "Combined response", resp.Content[0].Text)
	})
}

func TestAnthropicToolCalling_RoundTrip(t *testing.T) {
	translator := mustNewTranslator(createIntegrationTestLogger(), createTestConfig())
	ctx := context.Background()

	t.Run("single_tool_definition", func(t *testing.T) {
		// 1. Create request with tool definition
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{
					Role:    "user",
					Content: "What's the weather in Melbourne?",
				},
			},
			Tools: []AnthropicTool{
				{
					Name:        "get_weather",
					Description: "Get current weather for a location",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"location": map[string]interface{}{
								"type":        "string",
								"description": "City name",
							},
							"unit": map[string]interface{}{
								"type": "string",
								"enum": []string{"celsius", "fahrenheit"},
							},
						},
						"required": []string{"location"},
					},
				},
			},
			ToolChoice: "auto",
		}

		// 2. Transform request to OpenAI
		httpReq := createHTTPRequest(t, anthropicReq)
		transformed, err := translator.TransformRequest(ctx, httpReq)
		require.NoError(t, err)

		// 3. Verify tool definition conversion
		tools, ok := transformed.OpenAIRequest["tools"].([]map[string]interface{})
		require.True(t, ok)
		require.Len(t, tools, 1)

		assert.Equal(t, "function", tools[0]["type"])
		function := tools[0]["function"].(map[string]interface{})
		assert.Equal(t, "get_weather", function["name"])
		assert.Equal(t, "Get current weather for a location", function["description"])

		// Verify input_schema mapped to parameters
		parameters := function["parameters"].(map[string]interface{})
		assert.Equal(t, "object", parameters["type"])
		properties := parameters["properties"].(map[string]interface{})
		assert.Contains(t, properties, "location")

		// Verify tool_choice mapping
		assert.Equal(t, "auto", transformed.OpenAIRequest["tool_choice"])

		// 4. Simulate backend response with tool call
		toolCall := map[string]interface{}{
			"id":   "call_weather_123",
			"type": "function",
			"function": map[string]interface{}{
				"name":      "get_weather",
				"arguments": `{"location":"Melbourne","unit":"celsius"}`,
			},
		}
		backendResp := simulateBackendResponse(
			"Let me check the weather for you.",
			[]map[string]interface{}{toolCall},
			"tool_calls",
		)

		// 5. Transform response to Anthropic
		anthropicResp, err := translator.TransformResponse(ctx, backendResp, httpReq)
		require.NoError(t, err)

		// 6. Verify tool_use block creation
		resp := anthropicResp.(AnthropicResponse)
		assert.Equal(t, "tool_use", resp.StopReason)

		// When text content and tool calls are present, we should have both blocks
		// Find text and tool_use blocks
		var textBlock, toolBlock *ContentBlock
		for i := range resp.Content {
			switch resp.Content[i].Type {
			case "text":
				textBlock = &resp.Content[i]
			case "tool_use":
				toolBlock = &resp.Content[i]
			}
		}

		// Verify both blocks present
		require.NotNil(t, textBlock, "Should have text block")
		require.NotNil(t, toolBlock, "Should have tool_use block")

		// Verify text content
		assert.Equal(t, "Let me check the weather for you.", textBlock.Text)

		// Verify tool_use content
		assert.Equal(t, "call_weather_123", toolBlock.ID)
		assert.Equal(t, "get_weather", toolBlock.Name)

		// Verify input arguments parsed correctly
		require.NotNil(t, toolBlock.Input)
		assert.Equal(t, "Melbourne", toolBlock.Input["location"])
		assert.Equal(t, "celsius", toolBlock.Input["unit"])
	})

	t.Run("tool_result_round_trip", func(t *testing.T) {
		// Test sending tool result back to assistant
		anthropicReq := AnthropicRequest{
			Model:     ClaudeSonnetModel,
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				// Original user question
				{
					Role:    "user",
					Content: "What's the weather?",
				},
				// Assistant's tool call
				{
					Role: "assistant",
					Content: []interface{}{
						map[string]interface{}{
							"type": "tool_use",
							"id":   "toolu_weather_123",
							"name": "get_weather",
							"input": map[string]interface{}{
								"location": "Melbourne",
							},
						},
					},
				},
				// User provides tool result
				{
					Role: "user",
					Content: []interface{}{
						map[string]interface{}{
							"type":        "tool_result",
							"tool_use_id": "toolu_weather_123",
							"content":     "Temperature: 18°C, partly cloudy",
						},
					},
				},
			},
		}

