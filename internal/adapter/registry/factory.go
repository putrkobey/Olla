package registry

import (
	"context"
	"fmt"

	"github.com/putrkobey/Olla/internal/logger"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type RegistryConfig struct {
	Discovery       DiscoveryService             // injected dependency for discovery strategy
	UnificationConf *config.UnificationConfig    `yaml:"unification"`
	RoutingStrategy *config.ModelRoutingStrategy `yaml:"routing_strategy"`
	Type            string                       `yaml:"type"`
	EnableUnifier   bool                         `yaml:"enable_unifier"`
}

// DiscoveryService interface for discovery operations
type DiscoveryService interface {
	GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error)
	GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error)
	RefreshEndpoints(ctx context.Context) error
	UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error
}
