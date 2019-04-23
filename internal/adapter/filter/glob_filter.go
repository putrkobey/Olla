package filter

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/util/pattern"
)

// GlobFilter implements the Filter interface using glob pattern matching
type GlobFilter struct {
	// cache for compiled patterns to improve performance
	patternCache map[string]bool
	cacheMu      sync.RWMutex
}

// NewGlobFilter creates a new GlobFilter instance
func NewGlobFilter() ports.Filter {
	return &GlobFilter{
		patternCache: make(map[string]bool),
	}
}

// Apply filters a slice of items based on the filter configuration
func (f *GlobFilter) Apply(ctx context.Context, config *domain.FilterConfig, items interface{}, nameExtractor func(interface{}) string) (*domain.FilterResult, error) {
	// tried without reflection but it was slower
	// so we'll consider it for now
	itemsValue := reflect.ValueOf(items)
	if itemsValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("items must be a slice, got %T", items)
	}

	if config == nil || config.IsEmpty() {
		// no filtering needed, return all items as accepted
		return f.createResultFromItems(items, nil), nil
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid filter configuration: %w", err)
	}

	accepted := make([]interface{}, 0)
	rejected := make([]interface{}, 0)
