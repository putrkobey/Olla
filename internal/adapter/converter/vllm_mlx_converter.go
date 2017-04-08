package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type VLLMMLXResponse = profile.VLLMMLXResponse
type VLLMMLXModel = profile.VLLMMLXModel

// VLLMMLXConverter converts unified models to vLLM-MLX-compatible format.
// vLLM-MLX serves MLX models on Apple Silicon and uses the standard OpenAI-compatible list format.
type VLLMMLXConverter struct {
	*BaseConverter
}

// NewVLLMMLXConverter creates a new vLLM-MLX format converter.
func NewVLLMMLXConverter() ports.ModelResponseConverter {
	return &VLLMMLXConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeVLLMMLX),
	}
}

func (c *VLLMMLXConverter) GetFormatName() string {
	return constants.ProviderTypeVLLMMLX
}

func (c *VLLMMLXConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.VLLMMLXModel, 0, len(filtered))
	for _, model := range filtered {
		mlxModel := c.convertModel(model)
