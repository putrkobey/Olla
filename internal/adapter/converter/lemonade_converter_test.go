package converter

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestNewLemonadeConverter(t *testing.T) {
	converter := NewLemonadeConverter()
	assert.NotNil(t, converter)
	assert.Equal(t, constants.ProviderTypeLemonade, converter.GetFormatName())
}

func TestLemonadeConverter_GetFormatName(t *testing.T) {
	converter := NewLemonadeConverter()
	assert.Equal(t, constants.ProviderTypeLemonade, converter.GetFormatName())
}

func TestLemonadeConverter_ConvertToFormat_EmptyModels(t *testing.T) {
	converter := NewLemonadeConverter()
	result, err := converter.ConvertToFormat([]*domain.UnifiedModel{}, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.LemonadeResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	assert.Empty(t, response.Data)
}

func TestLemonadeConverter_ConvertToFormat_SingleModel(t *testing.T) {
	converter := NewLemonadeConverter()

	model := &domain.UnifiedModel{
		ID: "qwen/2.5:0.5b-instruct-cpu",
		Metadata: map[string]interface{}{
			"checkpoint": "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx",
			"recipe":     "oga-cpu",
		},
		Aliases: []domain.AliasEntry{
			{
				Name:   "Qwen2.5-0.5B-Instruct-CPU",
				Source: constants.ProviderTypeLemonade,
			},
		},
	}

	result, err := converter.ConvertToFormat([]*domain.UnifiedModel{model}, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.LemonadeResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 1)

	lemonadeModel := response.Data[0]
	assert.Equal(t, "Qwen2.5-0.5B-Instruct-CPU", lemonadeModel.ID)
	assert.Equal(t, "model", lemonadeModel.Object)
	assert.NotZero(t, lemonadeModel.Created)
	// "Qwen2.5" is picked up by the shared hyphenated known-org fallback
	// (ExtractOwnerFromModelID) rather than defaulting to "lemonade".
	assert.Equal(t, "Qwen2.5", lemonadeModel.OwnedBy)
	assert.Equal(t, "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx", lemonadeModel.Checkpoint)
	assert.Equal(t, "oga-cpu", lemonadeModel.Recipe)
}

func TestLemonadeConverter_ConvertToFormat_MultipleModels(t *testing.T) {
	converter := NewLemonadeConverter()

	models := []*domain.UnifiedModel{
		{
			ID: "qwen/2.5:0.5b-instruct-cpu",
			Metadata: map[string]interface{}{
				"checkpoint": "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx",
				"recipe":     "oga-cpu",
			},
			Aliases: []domain.AliasEntry{
				{Name: "Qwen2.5-0.5B-Instruct-CPU", Source: constants.ProviderTypeLemonade},
			},
		},
		{
			ID: "llama/3.2:1b-instruct-npu",
			Metadata: map[string]interface{}{
				"checkpoint": "meta-llama/Llama-3.2-1B-Instruct-ONNX",
				"recipe":     "oga-npu",
			},
			Aliases: []domain.AliasEntry{
				{Name: "Llama-3.2-1B-Instruct-NPU", Source: constants.ProviderTypeLemonade},
			},
		},
	}

	result, err := converter.ConvertToFormat(models, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.LemonadeResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 2)

	// Check first model (CPU)
	model1 := response.Data[0]
	assert.Equal(t, "Qwen2.5-0.5B-Instruct-CPU", model1.ID)
	// "Qwen2.5" is picked up by the shared hyphenated known-org fallback.
	assert.Equal(t, "Qwen2.5", model1.OwnedBy)
	assert.Equal(t, "amd/Qwen2.5-0.5B-Instruct-quantized_int4-float16-cpu-onnx", model1.Checkpoint)
	assert.Equal(t, "oga-cpu", model1.Recipe)

	// Check second model (NPU)
	model2 := response.Data[1]
	assert.Equal(t, "Llama-3.2-1B-Instruct-NPU", model2.ID)
	assert.Equal(t, constants.ProviderTypeLemonade, model2.OwnedBy)
	assert.Equal(t, "meta-llama/Llama-3.2-1B-Instruct-ONNX", model2.Checkpoint)
	assert.Equal(t, "oga-npu", model2.Recipe)
}

func TestLemonadeConverter_ConvertToFormat_DifferentRecipes(t *testing.T) {
	converter := NewLemonadeConverter()

	recipes := []string{"oga-cpu", "oga-npu", "oga-igpu", "llamacpp", "flm"}
	models := make([]*domain.UnifiedModel, len(recipes))

	for i, recipe := range recipes {
		models[i] = &domain.UnifiedModel{
			ID: "test-model-" + recipe,
			Metadata: map[string]interface{}{
				"checkpoint": "test/checkpoint-" + recipe,
				"recipe":     recipe,
			},
			Aliases: []domain.AliasEntry{
				{Name: "test-model-" + recipe, Source: constants.ProviderTypeLemonade},
			},
		}
	}

	result, err := converter.ConvertToFormat(models, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.LemonadeResponse)
	require.True(t, ok)

	assert.Len(t, response.Data, len(recipes))

	// Verify each model has the correct recipe
	for i, expectedRecipe := range recipes {
		assert.Equal(t, expectedRecipe, response.Data[i].Recipe,
			"Recipe mismatch for model %d", i)
	}
}

func TestLemonadeConverter_ConvertToFormat_ModelWithCheckpointOnly(t *testing.T) {
	converter := NewLemonadeConverter()

	model := &domain.UnifiedModel{
		ID: "test-checkpoint-only",
		Metadata: map[string]interface{}{
			"checkpoint": "test/model-checkpoint",
		},
		Aliases: []domain.AliasEntry{
			{Name: "test-checkpoint-only", Source: constants.ProviderTypeLemonade},
