package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type SGLangModelResponse = profile.SGLangResponse
type SGLangModelData = profile.SGLangModel

// SGLangConverter converts models to SGLang-compatible format with extended metadata
type SGLangConverter struct {
	*BaseConverter
}

// NewSGLangConverter creates a new SGLang format converter
func NewSGLangConverter() ports.ModelResponseConverter {
	return &SGLangConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeSGLang),
	}
}

func (c *SGLangConverter) GetFormatName() string {
	return constants.ProviderTypeSGLang
}

func (c *SGLangConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.SGLangModel, 0, len(filtered))
	for _, model := range filtered {
		sglangModel := c.convertModel(model)
		if sglangModel != nil {
			data = append(data, *sglangModel)
		}
	}

	return profile.SGLangResponse{
		Object: "list",
		Data:   data,
	}, nil
}

func (c *SGLangConverter) convertModel(model *domain.UnifiedModel) *profile.SGLangModel {
	// For SGLang, prefer the native SGLang name if available from source endpoints
	now := time.Now().Unix()
	modelID := c.FindNativeName(model)
	if modelID == "" {
		// Fallback to first alias or unified ID
		if len(model.Aliases) > 0 {
