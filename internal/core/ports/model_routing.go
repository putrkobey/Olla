package ports

import (
	"context"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// ModelRoutingStrategy defines how to route requests when models aren't available on all endpoints
type ModelRoutingStrategy interface {
	// GetRoutableEndpoints returns endpoints that should handle the model request
	GetRoutableEndpoints(
		ctx context.Context,
		modelName string,
		healthyEndpoints []*domain.Endpoint,
		modelEndpoints []string,
	) ([]*domain.Endpoint, *domain.ModelRoutingDecision, error)

	// Name returns the strategy name for logging
	Name() string
}

// routing decision actions
const (
	RoutingActionRouted   = "routed"
	RoutingActionFallback = "fallback"
	RoutingActionRejected = "rejected"
)

