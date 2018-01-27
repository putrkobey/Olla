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
// just returns healthy to simulate older behavior
type mockDiscoveryServiceOnlyHealthy struct {
	healthyEndpoints []*domain.Endpoint
}

func (m *mockDiscoveryServiceOnlyHealthy) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.healthyEndpoints, nil
}

func (m *mockDiscoveryServiceOnlyHealthy) RefreshEndpoints(ctx context.Context) error {
	return nil
}

func (m *mockDiscoveryServiceOnlyHealthy) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.healthyEndpoints, nil
}

func (m *mockDiscoveryServiceOnlyHealthy) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	return nil
}

func TestDiscoveryServiceAdapter_GetEndpoints(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all endpoints when available", func(t *testing.T) {
		allEndpoints := []*domain.Endpoint{
			{Name: "ep1", Status: domain.StatusHealthy},
			{Name: "ep2", Status: domain.StatusUnhealthy},
			{Name: "ep3", Status: domain.StatusOffline},
		}
		healthyOnly := []*domain.Endpoint{
			{Name: "ep1", Status: domain.StatusHealthy},
		}

		mock := &mockDiscoveryService{
			endpoints:        allEndpoints,
			healthyEndpoints: healthyOnly,
		}

		adapter := &discoveryServiceAdapter{discovery: mock}

		result, err := adapter.GetEndpoints(ctx)
		assert.NoError(t, err)
		assert.Equal(t, allEndpoints, result)
	})

	t.Run("uses type assertion correctly", func(t *testing.T) {
		healthyOnly := []*domain.Endpoint{
			{Name: "ep1", Status: domain.StatusHealthy},
		}

		// This mock implements the interface but adapter should
		// detect it has GetEndpoints and use it
		mock := &mockDiscoveryServiceOnlyHealthy{
			healthyEndpoints: healthyOnly,
		}

		adapter := &discoveryServiceAdapter{discovery: mock}

		result, err := adapter.GetEndpoints(ctx)
		assert.NoError(t, err)
		assert.Equal(t, healthyOnly, result)
	})
}

func TestDiscoveryServiceAdapter_GetHealthyEndpoints(t *testing.T) {
	ctx := context.Background()

	healthyEndpoints := []*domain.Endpoint{
		{Name: "ep1", Status: domain.StatusHealthy},
	}

	mock := &mockDiscoveryService{
		healthyEndpoints: healthyEndpoints,
	}

	adapter := &discoveryServiceAdapter{discovery: mock}
