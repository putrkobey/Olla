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
