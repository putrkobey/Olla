package discovery

import (
	"context"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// ModelDiscoveryClient handles HTTP-based model discovery for different llm frontends
type ModelDiscoveryClient interface {

	// DiscoverModels discovers available models from an endpoint using its platform profile
	DiscoverModels(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error)

	// HealthCheck does a health check geared more towards model discovery endpoints
	HealthCheck(ctx context.Context, endpoint *domain.Endpoint) error

	// GetMetrics metrics for discovery operations
	GetMetrics() DiscoveryMetrics
