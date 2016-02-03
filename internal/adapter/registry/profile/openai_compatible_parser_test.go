package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIParser_Parse(t *testing.T) {
	t.Parallel()

	parser := &openAIParser{}

	t.Run("captures max_model_len into MaxContextLength", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
			"data": [
				{
					"id": "mlx-community/Qwen2.5-7B-Instruct-4bit",
					"object": "model",
					"created": 1700000000,
					"owned_by": "omlx",
					"max_model_len": 32768
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		m := models[0]
		assert.Equal(t, "mlx-community/Qwen2.5-7B-Instruct-4bit", m.Name)
		require.NotNil(t, m.Details)
		require.NotNil(t, m.Details.MaxContextLength)
		assert.Equal(t, int64(32768), *m.Details.MaxContextLength)
	})

	t.Run("max_model_len coexists with created timestamp", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
			"data": [
				{
					"id": "some-model",
					"object": "model",
					"created": 1677610602,
					"owned_by": "provider",
					"max_model_len": 131072
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		m := models[0]
		require.NotNil(t, m.Details)
		require.NotNil(t, m.Details.MaxContextLength)
		assert.Equal(t, int64(131072), *m.Details.MaxContextLength)
		// created timestamp must still be mapped
		require.NotNil(t, m.Details.ModifiedAt)
		assert.Equal(t, time.Unix(1677610602, 0), *m.Details.ModifiedAt)
	})

	t.Run("absence of max_model_len leaves details unaffected", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
