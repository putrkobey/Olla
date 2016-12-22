package profile

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVLLMParser_Parse(t *testing.T) {
	parser := &vllmParser{}

	t.Run("parses valid vLLM response with full metadata", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
					"object": "model",
					"created": 1754535984,
					"owned_by": "vllm",
					"root": "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
					"parent": null,
					"max_model_len": 2048,
					"permission": [
						{
							"id": "modelperm-abc123",
							"object": "model_permission",
							"created": 1754535984,
							"allow_create_engine": false,
							"allow_sampling": true,
							"allow_logprobs": true,
							"allow_search_indices": false,
							"allow_view": true,
							"allow_fine_tuning": false,
							"organization": "*",
							"group": null,
							"is_blocking": false
						}
					]
				},
				{
					"id": "meta-llama/Meta-Llama-3.1-8B-Instruct",
					"object": "model",
					"created": 1754535985,
					"owned_by": "meta-llama",
					"root": "meta-llama/Meta-Llama-3.1-8B-Instruct",
					"parent": null,
					"max_model_len": 131072,
					"permission": []
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 2)

		// Verify first model (TinyLlama)
		tinyllama := models[0]
		assert.Equal(t, "TinyLlama/TinyLlama-1.1B-Chat-v1.0", tinyllama.Name)
		assert.Empty(t, tinyllama.Type, "provider names are not model kinds")
		assert.NotNil(t, tinyllama.Details)

		// Check max context length is captured
		require.NotNil(t, tinyllama.Details.MaxContextLength)
		assert.Equal(t, int64(2048), *tinyllama.Details.MaxContextLength)

		// Check creation time is converted
		require.NotNil(t, tinyllama.Details.ModifiedAt)
		assert.Equal(t, time.Unix(1754535984, 0), *tinyllama.Details.ModifiedAt)

		// Verify second model (Llama 3.1)
		llama := models[1]
		assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", llama.Name)
		assert.Empty(t, llama.Type, "provider names are not model kinds")
		assert.NotNil(t, llama.Details)

		// Check larger context length
		require.NotNil(t, llama.Details.MaxContextLength)
		assert.Equal(t, int64(131072), *llama.Details.MaxContextLength)

		// Check publisher is extracted from owned_by when not "vllm"
		require.NotNil(t, llama.Details.Publisher)
		assert.Equal(t, "meta-llama", *llama.Details.Publisher)
	})

	t.Run("handles models with fine-tuning parent", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "custom/fine-tuned-model",
					"object": "model",
					"created": 1754535986,
					"owned_by": "custom-org",
					"root": "base-model",
					"parent": "meta-llama/Meta-Llama-3.1-8B-Instruct",
					"max_model_len": 8192
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "custom/fine-tuned-model", model.Name)
		assert.NotNil(t, model.Details)

		// Check parent model is captured
		require.NotNil(t, model.Details.ParentModel)
		assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", *model.Details.ParentModel)

		// Check publisher
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "custom-org", *model.Details.Publisher)
	})

	t.Run("handles models without optional fields", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "simple-model",
					"object": "model",
					"created": 0,
					"owned_by": "vllm"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "simple-model", model.Name)
		assert.Empty(t, model.Type, "provider names are not model kinds")

		// Details should be nil when no metadata is present
		assert.Nil(t, model.Details)
	})

	t.Run("skips models without ID", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"object": "model",
					"created": 1754535987,
