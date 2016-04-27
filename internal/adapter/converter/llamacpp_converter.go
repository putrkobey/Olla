package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type LlamaCppResponse = profile.LlamaCppResponse
type LlamaCppModel = profile.LlamaCppModel

// LlamaCppConverter converts models to llama.cpp OpenAI-compatible format
type LlamaCppConverter struct {
	*BaseConverter
}

// NewLlamaCppConverter creates a new llama.cpp format converter
func NewLlamaCppConverter() ports.ModelResponseConverter {
	return &LlamaCppConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeLlamaCpp),
	}
}

func (c *LlamaCppConverter) GetFormatName() string {
	return constants.ProviderTypeLlamaCpp
}

func (c *LlamaCppConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.LlamaCppModel, 0, len(filtered))
	for _, model := range filtered {
		llamaModel := c.convertModel(model)
		if llamaModel != nil {
			data = append(data, *llamaModel)
		}
	}

	return profile.LlamaCppResponse{
		Object: "list",
		Data:   data,
	}, nil
}

func (c *LlamaCppConverter) convertModel(model *domain.UnifiedModel) *profile.LlamaCppModel {
	// llama.cpp uses simple OpenAI-compatible format with minimal fields
	now := time.Now().Unix()
	modelID := c.FindNativeName(model)
	if modelID == "" {
		// Fallback to first alias or unified ID
		if len(model.Aliases) > 0 {
			modelID = model.Aliases[0].Name
		} else {
			modelID = model.ID
		}
	}

