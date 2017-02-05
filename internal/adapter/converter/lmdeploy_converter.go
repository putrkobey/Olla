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
type LMDeployModelResponse = profile.LMDeployResponse
type LMDeployModelData = profile.LMDeployModel

// LMDeployConverter converts models to LMDeploy-compatible format.
// LMDeploy's /v1/models shape is OpenAI-compatible but without max_model_len
// and with owned_by defaulting to "lmdeploy".
type LMDeployConverter struct {
	*BaseConverter
}

// NewLMDeployConverter creates a new LMDeploy format converter.
func NewLMDeployConverter() ports.ModelResponseConverter {
	return &LMDeployConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeLMDeploy),
	}
}

func (c *LMDeployConverter) GetFormatName() string {
	return constants.ProviderTypeLMDeploy
}

func (c *LMDeployConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.LMDeployModel, 0, len(filtered))
	for _, model := range filtered {
		m := c.convertModel(model)
		if m != nil {
			data = append(data, *m)
		}
	}

	return profile.LMDeployResponse{
		Object: "list",
		Data:   data,
	}, nil
