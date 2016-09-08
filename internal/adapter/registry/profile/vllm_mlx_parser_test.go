package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestVLLMMLXParser_Parse(t *testing.T) {
	t.Parallel()

	parser := &vllmMLXParser{}

	t.Run("parses valid response with single model", func(t *testing.T) {
		t.Parallel()
		response := `{
			"object": "list",
			"data": [
				{
					"id": "mlx-community/Llama-3.2-3B-Instruct-4bit",
					"object": "model",
					"created": 1734000000,
					"owned_by": "mlx-community"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "mlx-community/Llama-3.2-3B-Instruct-4bit", model.Name)
		assert.Empty(t, model.Type, "provider names are not model kinds")
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.ModifiedAt)
		assert.Equal(t, int64(1734000000), model.Details.ModifiedAt.Unix())

		// "mlx-community" is a real HuggingFace organisation — publisher attribution should be preserved.
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "mlx-community", *model.Details.Publisher)

		require.NotNil(t, model.Details.Format)
		assert.Equal(t, constants.RecipeMLX, *model.Details.Format)
	})

	t.Run("parses multiple models", func(t *testing.T) {
		t.Parallel()
		response := `{
			"object": "list",
			"data": [
				{
					"id": "mlx-community/Llama-3.2-3B-Instruct-4bit",
					"object": "model",
					"created": 1734000000,
					"owned_by": "mlx-community"
				},
				{
					"id": "mlx-community/Qwen3-4B-4bit",
					"object": "model",
					"created": 1734000001,
					"owned_by": "mlx-community"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 2)

		assert.Equal(t, "mlx-community/Llama-3.2-3B-Instruct-4bit", models[0].Name)
		assert.Equal(t, "mlx-community/Qwen3-4B-4bit", models[1].Name)
	})

	t.Run("handles empty byte slice", func(t *testing.T) {
