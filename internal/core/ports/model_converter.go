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

// ModelFilters contains filtering options for model queries
type ModelFilters struct {
	Endpoint  string // Filter by specific endpoint name
	Available *bool  // nil = all, true = available only, false = unavailable only
	Family    string // Filter by model family (e.g., "llama", "phi")
	Type      string // Filter by model kind (e.g., "llm", "vlm", "embeddings"); exact match on the reported kind
