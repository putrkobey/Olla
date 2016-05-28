package ports

import (
	"fmt"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// ModelResponseConverter converts unified models to different API response formats
type ModelResponseConverter interface {
	// ConvertToFormat converts unified models to the specified format with optional filters
	ConvertToFormat(models []*domain.UnifiedModel, filters ModelFilters) (interface{}, error)
	// GetFormatName returns the name of the format this converter handles
	GetFormatName() string
}
