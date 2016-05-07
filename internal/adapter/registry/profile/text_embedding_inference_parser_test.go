package profile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// teiInfoSample mirrors a real TEI /info payload: the served name is the
// Hugging Face repo ID while model_id is the on-disk checkpoint path.
const teiInfoSample = `{
	"model_id": "/data/snowflake-arctic-embed-l-v2.0",
	"model_sha": null,
	"model_dtype": "float16",
	"served_model_name": "Snowflake/snowflake-arctic-embed-l-v2.0",
	"max_input_length": 8192,
	"model_type": {
		"embedding": {
			"pooling": "cls"
		}
	}
}`

func TestTextEmbeddingsInferenceParser_Parse(t *testing.T) {
	t.Parallel()

	parser := &textEmbeddingInferenceParser{}

	t.Run("prefers served_model_name over model_id", func(t *testing.T) {
		t.Parallel()

		models, err := parser.Parse([]byte(teiInfoSample))
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "Snowflake/snowflake-arctic-embed-l-v2.0", models[0].Name)
		assert.Equal(t, "embeddings", models[0].Type, "TEI models must be typed as embeddings for capability filtering")
	})

	t.Run("falls back to model_id when served_model_name is absent", func(t *testing.T) {
		t.Parallel()

		response := `{"model_id": "/data/sentence-transformers/all-MiniLM-L6-v2"}`
		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "/data/sentence-transformers/all-MiniLM-L6-v2", models[0].Name)
	})

	t.Run("falls back to model_id when served_model_name is empty", func(t *testing.T) {
		t.Parallel()

		response := `{"model_id": "/data/nomic-embed-text-v1.5", "served_model_name": ""}`
		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "/data/nomic-embed-text-v1.5", models[0].Name)
	})

	t.Run("records embedding model_type in details", func(t *testing.T) {
		t.Parallel()

		models, err := parser.Parse([]byte(`{"model_id": "/data/embed", "model_type": {"embedding": {"pooling": "cls"}}}`))
		require.NoError(t, err)
		require.Len(t, models, 1)
		require.NotNil(t, models[0].Details)
		require.NotNil(t, models[0].Details.Type)
		assert.Equal(t, "embeddings", *models[0].Details.Type)
		assert.Equal(t, "embeddings", models[0].Type)
	})

	t.Run("records reranker model_type in details", func(t *testing.T) {
