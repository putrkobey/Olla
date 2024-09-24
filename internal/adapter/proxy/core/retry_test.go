package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// testDiscoveryService for testing
type testDiscoveryService struct {
	updatedEndpoint *domain.Endpoint
}

func (t *testDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (t *testDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (t *testDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	return nil
}

func (t *testDiscoveryService) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	// Capture the endpoint for verification
	t.updatedEndpoint = &domain.Endpoint{}
	*t.updatedEndpoint = *endpoint
	return nil
}
