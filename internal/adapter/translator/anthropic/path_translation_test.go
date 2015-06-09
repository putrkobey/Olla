package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPathTranslation verifies that the translator sets the correct target path
// This ensures requests are proxied to the correct OpenAI endpoint
func TestPathTranslation(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())
	ctx := context.Background()

	tests := []struct {
		name               string
		anthropicReq       AnthropicRequest
		expectedTargetPath string
	}{
		{
			name: "non_streaming_message",
			anthropicReq: AnthropicRequest{
				Model:     "claude-sonnet-4-20250929",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
				Stream: false,
			},
			expectedTargetPath: "/v1/chat/completions",
		},
		{
			name: "streaming_message",
			anthropicReq: AnthropicRequest{
				Model:     "claude-sonnet-4-20250929",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{Role: "user", Content: "Hello"},
				},
				Stream: true,
			},
			expectedTargetPath: "/v1/chat/completions",
		},
		{
			name: "message_with_tools",
			anthropicReq: AnthropicRequest{
				Model:     "claude-sonnet-4-20250929",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{Role: "user", Content: "What's the weather?"},
				},
				Tools: []AnthropicTool{
					{
						Name:        "get_weather",
						Description: "Get weather",
						InputSchema: map[string]interface{}{"type": "object"},
					},
				},
			},
			expectedTargetPath: "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal request to JSON
			body, err := json.Marshal(tt.anthropicReq)
			require.NoError(t, err)

			// Create HTTP request
			req, err := http.NewRequest("POST", "/olla/anthropic/v1/messages", bytes.NewReader(body))
			require.NoError(t, err)

			// Transform request
			transformed, err := translator.TransformRequest(ctx, req)
			require.NoError(t, err)

