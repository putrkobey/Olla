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
	"github.com/putrkobey/Olla/internal/logger"
)

// createTestLogger creates a logger for testing
func createTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func TestTransformRequest_SimpleMessage(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Hello, world!",
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
	assert.Equal(t, "claude-3-5-sonnet-20241022", result.ModelName)
	assert.False(t, result.IsStreaming)
	assert.Equal(t, "anthropic", result.Metadata["format"])

	openaiReq := result.OpenAIRequest
	assert.Equal(t, "claude-3-5-sonnet-20241022", openaiReq["model"])
	assert.Equal(t, 1024, openaiReq["max_tokens"])
	assert.Equal(t, false, openaiReq["stream"])

	messages, ok := openaiReq["messages"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, messages, 1)
	assert.Equal(t, "user", messages[0]["role"])
	assert.Equal(t, "Hello, world!", messages[0]["content"])
}

func TestTransformRequest_WithSystemPrompt(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		System:    "You are a helpful assistant",
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

	// First message should be system
	assert.Equal(t, "system", messages[0]["role"])
	assert.Equal(t, "You are a helpful assistant", messages[0]["content"])

	// Second message should be user
	assert.Equal(t, "user", messages[1]["role"])
	assert.Equal(t, "Hello", messages[1]["content"])
}

func TestTransformRequest_WithTools(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "City name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		ToolChoice: "auto",
	}

	body, err := json.Marshal(anthropicReq)
	require.NoError(t, err)

	req := &http.Request{
		Body: io.NopCloser(bytes.NewReader(body)),
	}

	result, err := translator.TransformRequest(context.Background(), req)
	require.NoError(t, err)

	tools, ok := result.OpenAIRequest["tools"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, tools, 1)

	assert.Equal(t, "function", tools[0]["type"])
	function, ok := tools[0]["function"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "get_weather", function["name"])
	assert.Equal(t, "Get weather information", function["description"])

	parameters, ok := function["parameters"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "object", parameters["type"])

	toolChoice := result.OpenAIRequest["tool_choice"]
	assert.Equal(t, "auto", toolChoice)
}

func TestTransformRequest_MultipleTools(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	anthropicReq := AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 1024,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: "Check the weather and time",
			},
		},
		Tools: []AnthropicTool{
			{
				Name:        "get_weather",
				Description: "Get weather information",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
			{
				Name:        "get_time",
				Description: "Get current time",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
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

	tools, ok := result.OpenAIRequest["tools"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, tools, 2)

	assert.Equal(t, "function", tools[0]["type"])
	function0 := tools[0]["function"].(map[string]interface{})
	assert.Equal(t, "get_weather", function0["name"])

	assert.Equal(t, "function", tools[1]["type"])
	function1 := tools[1]["function"].(map[string]interface{})
	assert.Equal(t, "get_time", function1["name"])
}

func TestConvertToolChoice(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	testCases := []struct {
		name            string
		anthropicChoice interface{}
		expectedChoice  interface{}
		description     string
	}{
		{
			name:            "string_auto",
			anthropicChoice: "auto",
			expectedChoice:  "auto",
			description:     "auto maps to auto",
		},
		{
			name:            "string_any",
			anthropicChoice: "any",
			expectedChoice:  "required",
			description:     "any maps to required",
		},
		{
			name:            "string_none",
			anthropicChoice: "none",
			expectedChoice:  "none",
			description:     "none maps to none",
		},
		{
			name:            "object_auto",
			anthropicChoice: map[string]interface{}{"type": "auto"},
			expectedChoice:  "auto",
			description:     "object form with auto",
		},
		{
			name:            "object_any",
			anthropicChoice: map[string]interface{}{"type": "any"},
			expectedChoice:  "required",
			description:     "object form with any",
		},
		{
			name: "object_tool",
			anthropicChoice: map[string]interface{}{
				"type": "tool",
				"name": "get_weather",
			},
			expectedChoice: map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name": "get_weather",
				},
			},
			description: "specific tool selection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := translator.convertToolChoice(tc.anthropicChoice)
			require.NoError(t, err, tc.description)
			assert.Equal(t, tc.expectedChoice, result, tc.description)
		})
	}
}

func TestConvertToolChoice_EdgeCases(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	t.Run("unknown_string_defaults_to_auto", func(t *testing.T) {
		result, err := translator.convertToolChoice("unknown")
