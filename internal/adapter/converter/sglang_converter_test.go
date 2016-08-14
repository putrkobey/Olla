package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestNewSGLangConverter(t *testing.T) {
	converter := NewSGLangConverter()
	assert.NotNil(t, converter)
	assert.Equal(t, constants.ProviderTypeSGLang, converter.GetFormatName())
}

func TestSGLangConverter_ConvertToFormat_EmptyModels(t *testing.T) {
	converter := NewSGLangConverter()
	result, err := converter.ConvertToFormat([]*domain.UnifiedModel{}, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.SGLangResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	assert.Empty(t, response.Data)
}

func TestSGLangConverter_ConvertToFormat_SingleModel(t *testing.T) {
	converter := NewSGLangConverter()

	model := &domain.UnifiedModel{
		ID:               "meta-llama/Meta-Llama-3.1-8B-Instruct",
		MaxContextLength: func() *int64 { v := int64(131072); return &v }(),
		Aliases: []domain.AliasEntry{
			{
				Name:   "meta-llama/Meta-Llama-3.1-8B-Instruct",
				Source: constants.ProviderTypeSGLang,
			},
		},
	}

	result, err := converter.ConvertToFormat([]*domain.UnifiedModel{model}, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.SGLangResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 1)

	sglangModel := response.Data[0]
	assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", sglangModel.ID)
	assert.Equal(t, "model", sglangModel.Object)
	assert.NotZero(t, sglangModel.Created)
	assert.Equal(t, "meta-llama", sglangModel.OwnedBy)
	assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", sglangModel.Root)
	assert.NotNil(t, sglangModel.MaxModelLen)
	assert.Equal(t, int64(131072), *sglangModel.MaxModelLen)
}

func TestSGLangConverter_ConvertToFormat_MultipleModels(t *testing.T) {
	converter := NewSGLangConverter()

	models := []*domain.UnifiedModel{
		{
			ID:               "meta-llama/Meta-Llama-3.1-8B-Instruct",
			MaxContextLength: func() *int64 { v := int64(131072); return &v }(),
			Aliases: []domain.AliasEntry{
				{Name: "meta-llama/Meta-Llama-3.1-8B-Instruct", Source: constants.ProviderTypeSGLang},
			},
		},
		{
			ID:               "microsoft/DialoGPT-medium",
			MaxContextLength: func() *int64 { v := int64(8192); return &v }(),
			Aliases: []domain.AliasEntry{
				{Name: "microsoft/DialoGPT-medium", Source: constants.ProviderTypeSGLang},
			},
		},
	}

	result, err := converter.ConvertToFormat(models, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.SGLangResponse)
	require.True(t, ok)

	assert.Equal(t, "list", response.Object)
	require.Len(t, response.Data, 2)

	// Check first model
	model1 := response.Data[0]
	assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", model1.ID)
	assert.Equal(t, "meta-llama", model1.OwnedBy)
	assert.NotNil(t, model1.MaxModelLen)
	assert.Equal(t, int64(131072), *model1.MaxModelLen)

	// Check second model
	model2 := response.Data[1]
	assert.Equal(t, "microsoft/DialoGPT-medium", model2.ID)
	assert.Equal(t, "microsoft", model2.OwnedBy)
	assert.NotNil(t, model2.MaxModelLen)
	assert.Equal(t, int64(8192), *model2.MaxModelLen)
}

func TestSGLangConverter_ConvertToFormat_ModelWithVisionCapability(t *testing.T) {
	converter := NewSGLangConverter()

	model := &domain.UnifiedModel{
		ID:           "llava-hf/llava-1.5-13b-hf",
		Capabilities: []string{"vision", "chat"},
		Aliases: []domain.AliasEntry{
			{Name: "llava-hf/llava-1.5-13b-hf", Source: constants.ProviderTypeSGLang},
		},
	}

	result, err := converter.ConvertToFormat([]*domain.UnifiedModel{model}, ports.ModelFilters{})

	require.NoError(t, err)
	response, ok := result.(profile.SGLangResponse)
