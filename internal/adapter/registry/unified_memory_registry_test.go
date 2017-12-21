package registry

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Mock endpoint repository for testing
type mockEndpointRepository struct {
	endpoints []*domain.Endpoint
}

func (m *mockEndpointRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.endpoints, nil
}

func (m *mockEndpointRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	var routable []*domain.Endpoint
	for _, ep := range m.endpoints {
		if ep.Status.IsRoutable() {
			routable = append(routable, ep)
		}
	}
	return routable, nil
}

func (m *mockEndpointRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	var healthy []*domain.Endpoint
	for _, ep := range m.endpoints {
		if ep.Status == domain.StatusHealthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy, nil
}

func (m *mockEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	return nil
}

func (m *mockEndpointRepository) LoadFromConfig(ctx context.Context, configs []config.EndpointConfig) error {
	return nil
}

func (m *mockEndpointRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	return false
}

func createTestUnifiedRegistry() *UnifiedMemoryModelRegistry {
	return NewUnifiedMemoryModelRegistry(createTestLogger(), nil, nil, nil)
}

func TestGetHealthyEndpointsForModel(t *testing.T) {
	ctx := context.Background()
	registry := createTestUnifiedRegistry()

	// Create test endpoints
	endpoint1 := &domain.Endpoint{
		URLString: "http://localhost:11434",
		Name:      "ollama1",
		Status:    domain.StatusHealthy,
	}

	endpoint2 := &domain.Endpoint{
		URLString: "http://localhost:11435",
		Name:      "ollama2",
		Status:    domain.StatusHealthy,
	}

	endpoint3 := &domain.Endpoint{
		URLString: "http://localhost:11436",
		Name:      "ollama3",
		Status:    domain.StatusUnhealthy,
	}

	// Register endpoints
	registry.RegisterEndpoint(endpoint1)
	registry.RegisterEndpoint(endpoint2)
	registry.RegisterEndpoint(endpoint3)

	// Create mock endpoint repository
	mockRepo := &mockEndpointRepository{
		endpoints: []*domain.Endpoint{endpoint1, endpoint2, endpoint3},
	}

	// Register models for endpoints
	model1 := &domain.ModelInfo{
		Name:     "llama3:8b",
		LastSeen: time.Now(),
	}

	model2 := &domain.ModelInfo{
		Name:     "mistral:7b",
		LastSeen: time.Now(),
	}

	// Register models - llama3:8b on endpoints 1 and 2, mistral:7b only on endpoint 3
	err := registry.RegisterModel(ctx, endpoint1.URLString, model1)
	if err != nil {
		t.Fatalf("Failed to register model1 on endpoint1: %v", err)
	}

	err = registry.RegisterModel(ctx, endpoint2.URLString, model1)
