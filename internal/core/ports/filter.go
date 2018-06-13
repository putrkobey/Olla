package ports

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// Filter defines the interface for filtering items based on patterns
type Filter interface {
	// Apply filters a slice of items based on the filter configuration
