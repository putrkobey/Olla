package converter

import (
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// LMStudioModelResponse represents the LM Studio-compatible format response
type LMStudioModelResponse struct {
	Object string              `json:"object"`
	Data   []LMStudioModelData `json:"data"`
}

// LMStudioModelData represents a single model in LM Studio format
type LMStudioModelData struct {
	MaxContextLength *int64 `json:"max_context_length,omitempty"`
	ID               string `json:"id"`
	Object           string `json:"object"`
	Type             string `json:"type"`
	Publisher        string `json:"publisher,omitempty"`
	Arch             string `json:"arch,omitempty"`
	Quantization     string `json:"quantization,omitempty"`
	State            string `json:"state"`
}

// LMStudioConverter converts models to LM Studio-compatible format
type LMStudioConverter struct {
	*BaseConverter
}

// NewLMStudioConverter creates a new LM Studio format converter
func NewLMStudioConverter() ports.ModelResponseConverter {
	return &LMStudioConverter{
		BaseConverter: NewBaseConverter(constants.ProviderPrefixLMStudio1),
	}
}

func (c *LMStudioConverter) GetFormatName() string {
	return constants.ProviderPrefixLMStudio1
}

func (c *LMStudioConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
