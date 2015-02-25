package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type LemonadeModelResponse = profile.LemonadeResponse
type LemonadeModelData = profile.LemonadeModel

// LemonadeConverter converts models to Lemonade-compatible format with extended metadata
type LemonadeConverter struct {
	*BaseConverter
}

// NewLemonadeConverter creates a new Lemonade format converter
func NewLemonadeConverter() ports.ModelResponseConverter {
	return &LemonadeConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeLemonade),
	}
}

func (c *LemonadeConverter) GetFormatName() string {
	return constants.ProviderTypeLemonade
}

func (c *LemonadeConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]profile.LemonadeModel, 0, len(filtered))
	for _, model := range filtered {
		lemonadeModel := c.convertModel(model)
		if lemonadeModel != nil {
			data = append(data, *lemonadeModel)
		}
	}

	return profile.LemonadeResponse{
		Object: "list",
		Data:   data,
	}, nil
}

func (c *LemonadeConverter) convertModel(model *domain.UnifiedModel) *profile.LemonadeModel {
	// For Lemonade, prefer the native Lemonade name if available from source endpoints
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

	lemonadeModel := &profile.LemonadeModel{
		ID:      modelID,
		Object:  "model",
		Created: now,
		OwnedBy: c.determineOwner(modelID),
	}

	// Extract Lemonade-specific metadata features
	c.applyMetadataFeatures(model, lemonadeModel)

	return lemonadeModel
}

// determineOwner extracts the organisation from the model ID or defaults to "lemonade"
func (c *LemonadeConverter) determineOwner(modelID string) string {
	// Use shared owner extraction logic from BaseConverter
	// Handles slash-separated (org/model) and hyphenated (org-model) formats
