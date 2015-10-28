package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// Mock components for integration testing
type mockDiscoveryService struct {
	endpoints []*domain.Endpoint
}

func (m *mockDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.endpoints, nil
}

func (m *mockDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	var healthy []*domain.Endpoint
	for _, ep := range m.endpoints {
		if ep.Status == domain.StatusHealthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy, nil
}

func (m *mockDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	return nil
}

type mockEndpointSelector struct {
	selectFunc func(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error)
}

func (m *mockEndpointSelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if m.selectFunc != nil {
		return m.selectFunc(ctx, endpoints)
	}
	if len(endpoints) > 0 {
		return endpoints[0], nil
	}
	return nil, nil
}

func (m *mockEndpointSelector) Name() string {
	return "mock"
}

func (m *mockEndpointSelector) IncrementConnections(endpoint *domain.Endpoint) {}
func (m *mockEndpointSelector) DecrementConnections(endpoint *domain.Endpoint) {}

func TestModelRoutingIntegration(t *testing.T) {
	// Create test logger
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	styledLogger := logger.NewPlainStyledLogger(log)

	// Create unified registry
	unifiedRegistry := registry.NewUnifiedMemoryModelRegistry(styledLogger, nil, nil, nil)

	// Create test endpoints
	endpoint1, _ := url.Parse("http://localhost:11434")
	endpoint2, _ := url.Parse("http://localhost:11435")
	endpoint3, _ := url.Parse("http://localhost:11436")

	endpoints := []*domain.Endpoint{
		{
			Name:        "ollama1",
			URL:         endpoint1,
			URLString:   "http://localhost:11434",
			Status:      domain.StatusHealthy,
			Priority:    1,
			LastChecked: time.Now(),
		},
		{
			Name:        "ollama2",
			URL:         endpoint2,
			URLString:   "http://localhost:11435",
			Status:      domain.StatusHealthy,
			Priority:    2,
			LastChecked: time.Now(),
		},
		{
			Name:        "ollama3",
			URL:         endpoint3,
			URLString:   "http://localhost:11436",
			Status:      domain.StatusUnhealthy,
			Priority:    3,
			LastChecked: time.Now(),
		},
	}

	// Register endpoints in the registry
	for _, ep := range endpoints {
		unifiedRegistry.RegisterEndpoint(ep)
	}

	// Register models on endpoints
	ctx := context.Background()

	// llama3 available on endpoints 1 and 2
	llamaModel := &domain.ModelInfo{
		Name:     "llama3:8b",
		LastSeen: time.Now(),
	}
	unifiedRegistry.RegisterModel(ctx, endpoints[0].URLString, llamaModel)
	unifiedRegistry.RegisterModel(ctx, endpoints[1].URLString, llamaModel)

	// mistral only on unhealthy endpoint 3
	mistralModel := &domain.ModelInfo{
		Name:     "mistral:7b",
		LastSeen: time.Now(),
	}
	unifiedRegistry.RegisterModel(ctx, endpoints[2].URLString, mistralModel)

	// Create mock services
	discovery := &mockDiscoveryService{endpoints: endpoints}

	// Test cases
	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		setupSelector  func() *mockEndpointSelector
		description    string
	}{
		{
			name: "Route to healthy endpoint with model",
			requestBody: map[string]interface{}{
				"model":  "llama3:8b",
				"prompt": "Hello",
			},
			expectedStatus: http.StatusOK,
			setupSelector: func() *mockEndpointSelector {
				return &mockEndpointSelector{
					selectFunc: func(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
						// Should receive only healthy endpoints with the model
						if len(endpoints) != 2 {
							t.Errorf("Expected 2 endpoints, got %d", len(endpoints))
						}
						return endpoints[0], nil
					},
				}
			},
			description: "Should route llama3 requests to healthy endpoints",
		},
		{
			name: "Model only on unhealthy endpoint",
			requestBody: map[string]interface{}{
				"model":  "mistral:7b",
				"prompt": "Hello",
			},
			expectedStatus: http.StatusServiceUnavailable,
			setupSelector: func() *mockEndpointSelector {
