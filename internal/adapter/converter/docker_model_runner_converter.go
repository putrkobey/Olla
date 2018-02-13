package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type DockerModelRunnerResponse = profile.DockerModelRunnerResponse
type DockerModelRunnerModel = profile.DockerModelRunnerModel

// DockerModelRunnerConverter converts unified models to Docker Model Runner-compatible format
type DockerModelRunnerConverter struct {
	*BaseConverter
}

// NewDockerModelRunnerConverter creates a new Docker Model Runner format converter
func NewDockerModelRunnerConverter() ports.ModelResponseConverter {
	return &DockerModelRunnerConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeDockerMR),
	}
}

func (c *DockerModelRunnerConverter) GetFormatName() string {
	return constants.ProviderTypeDockerMR
}

func (c *DockerModelRunnerConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.DockerModelRunnerModel, 0, len(filtered))
	for _, model := range filtered {
		dmrModel := c.convertModel(model)
		if dmrModel != nil {
			data = append(data, *dmrModel)
		}
	}

	return profile.DockerModelRunnerResponse{
		Object: "list",
		Data:   data,
	}, nil
}

func (c *DockerModelRunnerConverter) convertModel(model *domain.UnifiedModel) *profile.DockerModelRunnerModel {
	now := time.Now().Unix()
	modelID := c.FindNativeName(model)
	if modelID == "" {
		// Fall back to first alias, then unified ID
		if len(model.Aliases) > 0 {
			modelID = model.Aliases[0].Name
