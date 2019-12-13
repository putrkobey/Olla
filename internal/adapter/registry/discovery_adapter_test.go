package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type mockDiscoveryService struct {
	endpoints        []*domain.Endpoint
	healthyEndpoints []*domain.Endpoint
	refreshCalled    bool
	updateCalled     bool
	lastUpdated      *domain.Endpoint
}

func (m *mockDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	m.refreshCalled = true
	return nil
}

func (m *mockDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.healthyEndpoints, nil
}

func (m *mockDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.endpoints, nil
}

func (m *mockDiscoveryService) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	m.updateCalled = true
	m.lastUpdated = endpoint
	return nil
}

// mockDiscoveryServiceOnlyHealthy implements the interface but GetEndpoints
