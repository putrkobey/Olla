package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestValidation_RequiredFields(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())
	ctx := context.Background()

	t.Run("missing_model", func(t *testing.T) {
		anthropicReq := AnthropicRequest{
			// Model: "", // missing required field
			MaxTokens: 1024,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "model field is required")
	})

	t.Run("missing_messages", func(t *testing.T) {
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: 1024,
			Messages:  []AnthropicMessage{}, // empty - required
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one message is required")
	})
}

func TestRequestValidation_ParameterRanges(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())
	ctx := context.Background()

	t.Run("negative_max_tokens", func(t *testing.T) {
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: -100, // invalid
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_tokens must be at least 1")
	})

	t.Run("zero_max_tokens", func(t *testing.T) {
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: 0, // invalid - must be at least 1
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max_tokens must be at least 1")
	})

	t.Run("temperature_too_high", func(t *testing.T) {
		temp := 3.0 // > 2.0
		anthropicReq := AnthropicRequest{
			Model:       "claude-sonnet-4-20250929",
			MaxTokens:   1024,
			Temperature: &temp,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "temperature must be between 0 and 2")
	})

	t.Run("temperature_negative", func(t *testing.T) {
		temp := -0.5
		anthropicReq := AnthropicRequest{
			Model:       "claude-sonnet-4-20250929",
			MaxTokens:   1024,
			Temperature: &temp,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "temperature must be between 0 and 2")
	})

	t.Run("top_p_too_high", func(t *testing.T) {
		topP := 1.5 // > 1.0
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: 1024,
			TopP:      &topP,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
	})

	t.Run("top_p_negative", func(t *testing.T) {
		topP := -0.1
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: 1024,
			TopP:      &topP,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}

		body, err := json.Marshal(anthropicReq)
		require.NoError(t, err)

		req := &http.Request{
			Body: io.NopCloser(bytes.NewReader(body)),
		}

		_, err = translator.TransformRequest(ctx, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "top_p must be between 0 and 1")
	})

	t.Run("top_k_negative", func(t *testing.T) {
		topK := -10
		anthropicReq := AnthropicRequest{
			Model:     "claude-sonnet-4-20250929",
			MaxTokens: 1024,
			TopK:      &topK,
			Messages: []AnthropicMessage{
				{Role: "user", Content: "Hello"},
			},
		}
