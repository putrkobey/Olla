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

func TestCountTokens(t *testing.T) {
	trans := mustNewTranslator(createTestLogger(), createTestConfig())

	tests := []struct {
		name          string
		request       AnthropicRequest
		expectedMin   int
		expectedMax   int
		expectedExact int
	}{
		{
			name: "simple string message",
			request: AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{
						Role:    "user",
						Content: "Hello, world!", // 13 chars = 3 tokens (13/4 = 3.25, truncated to 3)
					},
				},
			},
			expectedExact: 3,
		},
		{
			name: "message with system prompt",
			request: AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				System:    "You are a helpful assistant.", // 28 chars
				Messages: []AnthropicMessage{
					{
						Role:    "user",
						Content: "What is the weather?", // 20 chars
					},
				},
			},
			expectedExact: 12, // (28 + 20) / 4 = 12
		},
		{
			name: "multiple messages",
			request: AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{
						Role:    "user",
						Content: "First message", // 13 chars
					},
					{
						Role:    "assistant",
						Content: "Response", // 8 chars
					},
					{
						Role:    "user",
						Content: "Follow up", // 9 chars
					},
				},
			},
			expectedExact: 7, // (13 + 8 + 9) / 4 = 7.5, truncated to 7
		},
		{
			name: "content blocks with text",
			request: AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []ContentBlock{
							{
								Type: "text",
								Text: "This is a text block", // 20 chars
							},
						},
					},
				},
			},
			expectedExact: 5, // 20 / 4 = 5
		},
		{
			name: "complex content with tool use",
			request: AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 1024,
				Messages: []AnthropicMessage{
					{
						Role: "user",
						Content: []ContentBlock{
							{
								Type: "text",
								Text: "Use a tool", // 10 chars
							},
						},
					},
					{
						Role: "assistant",
						Content: []ContentBlock{
							{
								Type: "tool_use",
								ID:   "toolu_123",
								Name: "get_weather", // 11 chars
								Input: map[string]interface{}{
									"location": "Sydney", // JSON: {"location":"Sydney"} = 22 chars
								},
							},
						},
					},
				},
			},
			expectedMin: 10, // At least (10 + 11 + 22) / 4 = 10.75 = 10
			expectedMax: 11, // Could be 11 depending on rounding
