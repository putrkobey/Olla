package ports

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// ModelUnifier provides unified naming and aliasing for models across different platforms
type ModelUnifier interface {
	// UnifyModel converts a platform-specific model to unified format
	UnifyModel(ctx context.Context, sourceModel *domain.ModelInfo, endpoint *domain.Endpoint) (*domain.UnifiedModel, error)
