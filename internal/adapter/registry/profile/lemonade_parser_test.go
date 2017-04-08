package profile

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLemonadeParser_Parse(t *testing.T) {
	parser := &lemonadeParser{}

	t.Run("parses valid Lemonade response with full metadata", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "Qwen2.5-0.5B-Instruct-CPU",
					"object": "model",
					"created": 1759361710,
					"owned_by": "lemonade",
					"checkpoint": "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx",
					"recipe": "oga-cpu"
				},
				{
					"id": "Llama-3.2-1B-Instruct-NPU",
					"object": "model",
					"created": 1759361720,
					"owned_by": "lemonade",
					"checkpoint": "meta-llama/Llama-3.2-1B-Instruct-quantized-int4-npu-onnx",
					"recipe": "oga-npu"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 2)

		// Verify first model (Qwen CPU)
		qwen := models[0]
		assert.Equal(t, "Qwen2.5-0.5B-Instruct-CPU", qwen.Name)
		assert.Empty(t, qwen.Type, "provider names are not model kinds")
		assert.NotNil(t, qwen.Details)

		// Check creation time is converted
		require.NotNil(t, qwen.Details.ModifiedAt)
		assert.Equal(t, time.Unix(1759361710, 0), *qwen.Details.ModifiedAt)

		// Check publisher is extracted from checkpoint path
		require.NotNil(t, qwen.Details.Publisher)
		assert.Equal(t, "amd", *qwen.Details.Publisher)

		// Check format is inferred from recipe (oga-* = onnx)
		require.NotNil(t, qwen.Details.Format)
		assert.Equal(t, "onnx", *qwen.Details.Format)

		require.NotNil(t, qwen.Details.Checkpoint)
		assert.Equal(t, "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx", *qwen.Details.Checkpoint)

		require.NotNil(t, qwen.Details.Recipe)
		assert.Equal(t, "oga-cpu", *qwen.Details.Recipe)

		// Verify second model (Llama NPU)
		llama := models[1]
		assert.Equal(t, "Llama-3.2-1B-Instruct-NPU", llama.Name)
		assert.Empty(t, llama.Type, "provider names are not model kinds")
		assert.NotNil(t, llama.Details)

		// Check publisher extraction
		require.NotNil(t, llama.Details.Publisher)
		assert.Equal(t, "meta-llama", *llama.Details.Publisher)

		// Check format is still onnx for oga-npu recipe
		require.NotNil(t, llama.Details.Format)
		assert.Equal(t, "onnx", *llama.Details.Format)

		require.NotNil(t, llama.Details.Checkpoint)
		assert.Equal(t, "meta-llama/Llama-3.2-1B-Instruct-quantized-int4-npu-onnx", *llama.Details.Checkpoint)

		require.NotNil(t, llama.Details.Recipe)
		assert.Equal(t, "oga-npu", *llama.Details.Recipe)
	})

	t.Run("infers GGUF format from llamacpp recipe", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "Llama-3.2-3B-Q4-KM",
					"object": "model",
					"created": 1759361730,
					"owned_by": "lemonade",
					"checkpoint": "bartowski/Llama-3.2-3B-Instruct-GGUF",
					"recipe": "llamacpp"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "Llama-3.2-3B-Q4-KM", model.Name)

		// Check format is inferred as GGUF from llamacpp recipe
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, "gguf", *model.Details.Format)

		// Check publisher
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "bartowski", *model.Details.Publisher)
	})

	t.Run("infers GGUF format from flm recipe", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "Phi-3.5-Mini-FLM",
					"object": "model",
					"created": 1759361740,
					"owned_by": "lemonade",
					"checkpoint": "microsoft/Phi-3.5-mini-instruct-gguf",
					"recipe": "flm"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]

		// Check format is inferred as GGUF from flm recipe
		require.NotNil(t, model.Details)
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, "gguf", *model.Details.Format)

		// Check publisher
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "microsoft", *model.Details.Publisher)
	})

	t.Run("handles models without checkpoint path", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "local-model",
					"object": "model",
					"created": 1759361750,
					"owned_by": "lemonade",
					"checkpoint": "",
					"recipe": "oga-cpu"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "local-model", model.Name)

		// Publisher should not be set when checkpoint is empty
		require.NotNil(t, model.Details)
		assert.Nil(t, model.Details.Publisher)

		// Format should still be inferred from recipe
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, "onnx", *model.Details.Format)
	})

	t.Run("handles models with checkpoint but no slash", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "simple-model",
					"object": "model",
					"created": 1759361760,
					"owned_by": "lemonade",
					"checkpoint": "model-name-without-namespace",
					"recipe": "oga-igpu"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		// Publisher should not be extracted when there's no slash
		require.NotNil(t, model.Details)
		assert.Nil(t, model.Details.Publisher)

		// Format should be inferred from oga-igpu recipe
		require.NotNil(t, model.Details.Format)
		assert.Equal(t, "onnx", *model.Details.Format)
	})

	t.Run("handles models with unknown recipe", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "future-model",
					"object": "model",
					"created": 1759361770,
					"owned_by": "lemonade",
					"checkpoint": "vendor/future-engine-model",
					"recipe": "future-engine"
				}
			]
		}`

		models, err := parser.Parse([]byte(response))
		require.NoError(t, err)
		require.Len(t, models, 1)

		model := models[0]
		assert.Equal(t, "future-model", model.Name)

		// Format should not be inferred for unknown recipes
		require.NotNil(t, model.Details)
		assert.Nil(t, model.Details.Format)

		// Publisher should still be extracted
		require.NotNil(t, model.Details.Publisher)
		assert.Equal(t, "vendor", *model.Details.Publisher)
	})

	t.Run("handles models without optional fields", func(t *testing.T) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "minimal-model",
					"object": "model",
					"created": 0,
					"owned_by": "lemonade"
				}
			]
		}`

