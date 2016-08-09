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
	filtered := filterModels(models, filters)

	data := make([]profile.VLLMModel, 0, len(filtered))
	for _, model := range filtered {
		vllmModel := c.convertModel(model)
		if vllmModel != nil {
			data = append(data, *vllmModel)
		}
	}

	return profile.VLLMResponse{
		Object: "list",
		Data:   data,
	}, nil
}

func (c *VLLMConverter) convertModel(model *domain.UnifiedModel) *profile.VLLMModel {
	// For vLLM, prefer the native vLLM name if available from source endpoints
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

	vllmModel := &profile.VLLMModel{
		ID:      modelID,
		Object:  "model",
		Created: now,
		OwnedBy: c.determineOwner(modelID),
