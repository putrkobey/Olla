package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestDockerModelRunnerParser_Parse(t *testing.T) {
	t.Parallel()

	parser := &dockerModelRunnerParser{}

	t.Run("parses valid response with single model", func(t *testing.T) {
		t.Parallel()
		response := `{
			"object": "list",
			"data": [
				{
					"id": "ai/smollm2",
					"object": "model",
					"created": 1734000000,
					"owned_by": "docker"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "ai/smollm2", model.Name)
		assert.Empty(t, model.Type, "provider names are not model kinds")
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.ModifiedAt)
		assert.Equal(t, int64(1734000000), model.Details.ModifiedAt.Unix())
	})

	t.Run("parses multiple models", func(t *testing.T) {
		t.Parallel()
		response := `{
			"object": "list",
			"data": [
				{
					"id": "ai/smollm2",
					"object": "model",
					"created": 1734000000,
					"owned_by": "docker"
				},
				{
					"id": "docker/llama3.2",
					"object": "model",
					"created": 1734000001,
					"owned_by": "meta"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 2)

		assert.Equal(t, "ai/smollm2", models[0].Name)
		assert.Equal(t, "docker/llama3.2", models[1].Name)

		// "meta" is a real publisher — should be preserved
		require.NotNil(t, models[1].Details.Publisher)
		assert.Equal(t, "meta", *models[1].Details.Publisher)
	})

	t.Run("handles empty byte slice", func(t *testing.T) {
		t.Parallel()
		models, err := parser.Parse([]byte{})
		require.NoError(t, err)
		assert.Empty(t, models)
	})

	t.Run("handles empty data array", func(t *testing.T) {
		t.Parallel()
		response := `{"object": "list", "data": []}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		assert.Empty(t, models)
	})

	t.Run("skips models without ID", func(t *testing.T) {
		t.Parallel()
		response := `{
			"object": "list",
			"data": [
				{
					"object": "model",
					"created": 1734000002,
					"owned_by": "test"
				},
				{
					"id": "valid/model",
					"object": "model",
					"created": 1734000003,
					"owned_by": "test"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
