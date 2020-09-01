package profile

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestLlamaCppParser_Parse(t *testing.T) {
	parser := &llamaCppParser{}

	t.Run("parses standard llama.cpp response with single model", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "llama-3.1-8b-instruct-q4_k_m.gguf",
					"object": "model",
					"created": 1704067200,
					"owned_by": "meta-llama"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "llama-3.1-8b-instruct-q4_k_m.gguf", model.Name)
		assert.Empty(t, model.Type, "provider names are not model kinds")
		require.NotNil(t, model.Details)

		// Check timestamp is converted correctly
		require.NotNil(t, model.Details.ModifiedAt)
		assert.Equal(t, time.Unix(1704067200, 0), *model.Details.ModifiedAt)

		// Check publisher is extracted when not "llamacpp"
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "meta-llama", *model.Details.Publisher)

		// CRITICAL: GGUF format MUST always be set for llama.cpp models
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, constants.RecipeGGUF, *model.Details.Format)
	})

	t.Run("always sets GGUF format for all models", func(t *testing.T) {
		// Test multiple models to ensure EVERY model gets GGUF format
		// This is the most critical test - llama.cpp EXCLUSIVELY serves GGUF
		response := `{
			"object": "list",
			"data": [
				{
					"id": "mistral-7b-v0.1-q4_0.gguf",
					"object": "model",
					"created": 1704067200,
					"owned_by": "mistralai"
				},
				{
					"id": "tinyllama-1.1b-chat-v1.0.Q4_K_M.gguf",
					"object": "model",
					"created": 1704067300,
					"owned_by": "llamacpp"
				},
				{
					"id": "nomic-embed-text-v1.5.Q4_K_M.gguf",
					"object": "model",
					"created": 1704067400,
					"owned_by": "nomic-ai"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 3)

		// Verify EVERY single model has GGUF format set
		for i, model := range models {
			require.NotNil(t, model.Details, "Model %d (%s) should have Details", i, model.Name)
			require.NotNil(t, model.Details.Format, "Model %d (%s) must have Format set", i, model.Name)
			assert.Equal(t, constants.RecipeGGUF, *model.Details.Format, "Model %d (%s) must have GGUF format", i, model.Name)
		}
	})

	t.Run("skips publisher when owned_by is llamacpp", func(t *testing.T) {
		// "llamacpp" is the default value, not a real publisher
		response := `{
			"object": "list",
			"data": [
				{
					"id": "test-model.gguf",
					"object": "model",
					"created": 1704067200,
					"owned_by": "llamacpp"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		require.NotNil(t, model.Details)
		// Publisher should be nil when owned_by is "llamacpp"
		assert.Nil(t, model.Details.Publisher)
		// But format should still be set
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, constants.RecipeGGUF, *model.Details.Format)
	})

	t.Run("sets publisher when owned_by is organisation", func(t *testing.T) {
		testCases := []struct {
			ownedBy           string
			expectedPublisher string
		}{
			{"meta-llama", "meta-llama"},
			{"mistralai", "mistralai"},
			{"microsoft", "microsoft"},
			{"nomic-ai", "nomic-ai"},
		}

		for _, tc := range testCases {
			t.Run(tc.ownedBy, func(t *testing.T) {
				response := fmt.Sprintf(`{
					"object": "list",
					"data": [
						{
							"id": "test-model.gguf",
							"object": "model",
							"created": 1704067200,
							"owned_by": "%s"
						}
					]
				}`, tc.ownedBy)

				models, err := parser.Parse([]byte(response))
				require.NoError(t, err)
				require.Len(t, models, 1)

				model := models[0]
				require.NotNil(t, model.Details)
				require.NotNil(t, model.Details.Publisher)
				assert.Equal(t, tc.expectedPublisher, *model.Details.Publisher)
			})
		}
	})

	t.Run("handles empty response array", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": []
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		assert.Empty(t, models)
		assert.NotNil(t, models) // Should be empty slice, not nil
	})

	t.Run("handles empty data input", func(t *testing.T) {
		// Test with empty byte slice
		models, err := parser.Parse([]byte{})
		require.NoError(t, err)
		assert.Empty(t, models)
		assert.NotNil(t, models) // Should be empty slice, not nil
	})

	t.Run("returns error for malformed JSON", func(t *testing.T) {
		invalidJSON := `{
			"object": "list",
			"data": [
				{
					"id": "test-model.gguf",
					invalid json here
				}
			]
		}`

		models, err := parser.Parse([]byte(invalidJSON))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse llama.cpp response")
		assert.Nil(t, models)
	})

	t.Run("skips models with empty ID", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "",
					"object": "model",
					"created": 1704067200,
					"owned_by": "llamacpp"
				},
				{
					"id": "valid-model.gguf",
					"object": "model",
					"created": 1704067300,
					"owned_by": "llamacpp"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "valid-model.gguf", models[0].Name)
	})

	t.Run("preserves timestamps correctly", func(t *testing.T) {
		// Use specific timestamp (2024-01-01 00:00:00 UTC)
		timestamp := int64(1704067200)
		response := fmt.Sprintf(`{
			"object": "list",
			"data": [
				{
					"id": "test-model.gguf",
					"object": "model",
					"created": %d,
					"owned_by": "llamacpp"
				}
			]
		}`, timestamp)

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.ModifiedAt)
		assert.Equal(t, timestamp, model.Details.ModifiedAt.Unix())
	})

	t.Run("handles zero timestamp correctly", func(t *testing.T) {
		// Zero timestamps should NOT set ModifiedAt field
		response := `{
			"object": "list",
			"data": [
				{
					"id": "test-model.gguf",
					"object": "model",
					"created": 0,
					"owned_by": "llamacpp"
				}
			]
