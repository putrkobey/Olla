package converter

import (
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type VLLMModelResponse = profile.VLLMResponse
type VLLMModelData = profile.VLLMModel
type VLLMModelPermission = profile.VLLMModelPermission

// VLLMConverter converts models to vLLM-compatible format with extended metadata
type VLLMConverter struct {
	*BaseConverter
}

// NewVLLMConverter creates a new vLLM format converter
func NewVLLMConverter() ports.ModelResponseConverter {
	return &VLLMConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeVLLM),
	}
}

func (c *VLLMConverter) GetFormatName() string {
	return constants.ProviderTypeVLLM
}

func (c *VLLMConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
