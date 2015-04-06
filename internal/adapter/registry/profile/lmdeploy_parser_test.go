package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLMDeployParser_Parse(t *testing.T) {
	t.Parallel()

	parser := &lmdeployParser{}

	t.Run("parses valid response with full metadata", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
			"data": [
				{
					"id": "internlm/internlm2_5-7b-chat",
					"object": "model",
					"created": 1754535984,
					"owned_by": "lmdeploy",
					"root": "internlm/internlm2_5-7b-chat",
					"parent": null,
					"permission": []
				},
				{
					"id": "meta-llama/Meta-Llama-3.1-8B-Instruct",
					"object": "model",
					"created": 1754535985,
					"owned_by": "meta-llama",
					"root": "meta-llama/Meta-Llama-3.1-8B-Instruct",
					"parent": null,
					"permission": []
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 2)

		internlm := models[0]
		assert.Equal(t, "internlm/internlm2_5-7b-chat", internlm.Name)
		assert.Empty(t, internlm.Type, "provider names are not model kinds")
		// owned_by "lmdeploy" is the default — publisher should not be set
		require.NotNil(t, internlm.Details)
		assert.Nil(t, internlm.Details.Publisher)
		require.NotNil(t, internlm.Details.ModifiedAt)
		assert.Equal(t, time.Unix(1754535984, 0), *internlm.Details.ModifiedAt)

		llama := models[1]
		assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", llama.Name)
		assert.Empty(t, llama.Type, "provider names are not model kinds")
		require.NotNil(t, llama.Details)
		require.NotNil(t, llama.Details.Publisher)
		assert.Equal(t, "meta-llama", *llama.Details.Publisher)
	})

	t.Run("handles fine-tuned model with parent", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
			"data": [
				{
					"id": "custom/fine-tuned-internlm",
					"object": "model",
					"created": 1754535986,
					"owned_by": "custom-org",
					"parent": "internlm/internlm2_5-7b-chat"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.ParentModel)
		assert.Equal(t, "internlm/internlm2_5-7b-chat", *model.Details.ParentModel)
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "custom-org", *model.Details.Publisher)
	})

	t.Run("skips models without ID", func(t *testing.T) {
		t.Parallel()

		response := `{
			"object": "list",
			"data": [
				{
					"object": "model",
					"created": 1754535987,
					"owned_by": "lmdeploy"
				},
				{
					"id": "valid-model",
					"object": "model",
					"created": 1754535988,
					"owned_by": "lmdeploy"
				}
			]
		}`
