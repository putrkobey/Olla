package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// UnifiedModelResponse represents the unified Olla format response
type UnifiedModelResponse struct {
	Object string             `json:"object"`
	Data   []UnifiedModelData `json:"data"`
}

// UnifiedModelData represents a single model in the unified format
type UnifiedModelData struct {
	Olla    *OllaExtensions `json:"olla,omitempty"`
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	OwnedBy string          `json:"owned_by"`
	Created int64           `json:"created"`
}

// OllaExtensions contains Olla-specific model information
type OllaExtensions struct {
	MaxContextLength *int64           `json:"max_context_length,omitempty"`
	Family           string           `json:"family"`
	Variant          string           `json:"variant"`
	Type             string           `json:"type,omitempty"`
	ParameterSize    string           `json:"parameter_size"`
	Quantization     string           `json:"quantization"`
	PromptTemplateID string           `json:"prompt_template_id,omitempty"`
	Aliases          []string         `json:"aliases"`
	Availability     []EndpointStatus `json:"availability"`
	Capabilities     []string         `json:"capabilities"`
}

// EndpointStatus represents model availability on an endpoint
type EndpointStatus struct {
	Endpoint string `json:"endpoint"` // Endpoint name only (no URL for security)
	State    string `json:"state"`
}

// UnifiedConverter converts models to the default Olla unified format
type UnifiedConverter struct {
	*BaseConverter
}

// NewUnifiedConverter creates a new unified format converter
func NewUnifiedConverter() ports.ModelResponseConverter {
	return &UnifiedConverter{
		BaseConverter: NewBaseConverter("unified"),
	}
}

func (c *UnifiedConverter) GetFormatName() string {
	return "unified"
}

func (c *UnifiedConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]UnifiedModelData, 0, len(filtered))
	for _, model := range filtered {
		data = append(data, c.convertModel(model))
	}

	return UnifiedModelResponse{
		Object: "list",
		Data:   data,
