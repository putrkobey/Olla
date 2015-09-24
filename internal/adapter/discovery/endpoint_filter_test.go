package discovery

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestEndpointModelFilterPreservation(t *testing.T) {
	tests := []struct {
		name         string
		endpointCfg  config.EndpointConfig
		expectFilter bool
		filterRules  *domain.FilterConfig
	}{
		{
			name: "endpoint with model filter",
			endpointCfg: config.EndpointConfig{
				Name:           "test-endpoint",
				URL:            "http://localhost:11434",
				HealthCheckURL: "/",
				ModelURL:       "/api/tags",
				ModelFilter: &domain.FilterConfig{
					Exclude: []string{"*embed*", "nomic-*"},
				},
				CheckInterval: 5 * time.Second,
				CheckTimeout:  2 * time.Second,
			},
			expectFilter: true,
			filterRules: &domain.FilterConfig{
				Exclude: []string{"*embed*", "nomic-*"},
			},
		},
		{
			name: "endpoint without model filter",
			endpointCfg: config.EndpointConfig{
				Name:           "test-endpoint-2",
				URL:            "http://localhost:11435",
				HealthCheckURL: "/",
				ModelURL:       "/api/tags",
				ModelFilter:    nil,
				CheckInterval:  5 * time.Second,
				CheckTimeout:   2 * time.Second,
			},
			expectFilter: false,
			filterRules:  nil,
		},
		{
			name: "endpoint with include filter",
			endpointCfg: config.EndpointConfig{
				Name:           "test-endpoint-3",
				URL:            "http://localhost:11436",
				HealthCheckURL: "/",
				ModelURL:       "/api/tags",
				ModelFilter: &domain.FilterConfig{
					Include: []string{"llama*", "mistral*"},
				},
				CheckInterval: 5 * time.Second,
				CheckTimeout:  2 * time.Second,
			},
			expectFilter: true,
			filterRules: &domain.FilterConfig{
				Include: []string{"llama*", "mistral*"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			repo := NewStaticEndpointRepository()

			// Load config
			err := repo.LoadFromConfig(ctx, []config.EndpointConfig{tt.endpointCfg})
			require.NoError(t, err)

			// Get the endpoint
			endpoints, err := repo.GetAll(ctx)
			require.NoError(t, err)
			require.Len(t, endpoints, 1)

			endpoint := endpoints[0]

			// Verify filter preservation
			if tt.expectFilter {
				assert.NotNil(t, endpoint.ModelFilter, "ModelFilter should be preserved")
				assert.Equal(t, tt.filterRules.Include, endpoint.ModelFilter.Include)
				assert.Equal(t, tt.filterRules.Exclude, endpoint.ModelFilter.Exclude)
			} else {
				assert.Nil(t, endpoint.ModelFilter, "ModelFilter should be nil")
			}
		})
	}
}

func TestDiscoveryServiceEndpointFiltering(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         *domain.Endpoint
		discoveredModels []*domain.ModelInfo
		expectedModels   []string
	}{
		{
			name: "filter excludes embed models",
			endpoint: &domain.Endpoint{
				Name:      "test-endpoint",
				URLString: "http://localhost:11434",
				ModelFilter: &domain.FilterConfig{
					Exclude: []string{"*embed*", "bge-*"},
				},
			},
			discoveredModels: []*domain.ModelInfo{
				{Name: "llama3-8b"},
				{Name: "nomic-embed-text"},
				{Name: "bge-large"},
				{Name: "mistral-7b"},
				{Name: "text-embedding-ada"},
			},
			expectedModels: []string{"llama3-8b", "mistral-7b"},
		},
		{
			name: "filter includes only specific models",
			endpoint: &domain.Endpoint{
				Name:      "test-endpoint-2",
				URLString: "http://localhost:11435",
				ModelFilter: &domain.FilterConfig{
					Include: []string{"llama*", "mistral*"},
				},
			},
			discoveredModels: []*domain.ModelInfo{
				{Name: "llama3-8b"},
				{Name: "qwen2-7b"},
				{Name: "mistral-7b"},
				{Name: "deepseek-coder"},
				{Name: "llama2-13b"},
			},
			expectedModels: []string{"llama3-8b", "mistral-7b", "llama2-13b"},
		},
		{
			name: "no filter returns all models",
			endpoint: &domain.Endpoint{
				Name:        "test-endpoint-3",
				URLString:   "http://localhost:11436",
				ModelFilter: nil,
			},
			discoveredModels: []*domain.ModelInfo{
				{Name: "model1"},
				{Name: "model2"},
				{Name: "model3"},
			},
			expectedModels: []string{"model1", "model2", "model3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create mock components using a modified mockDiscoveryClient
			discoveredModels := tt.discoveredModels
			client := &mockDiscoveryClientWithModels{
				models: discoveredModels,
			}

			endpointRepo := &mockEndpointRepository{
				healthyEndpoints: []*domain.Endpoint{tt.endpoint},
			}

			registeredModels := make(map[string][]*domain.ModelInfo)
			modelRegistry := &mockModelRegistryWithCapture{
				registeredModels: registeredModels,
			}

			config := DiscoveryConfig{
				Interval:          30 * time.Second,
				Timeout:           10 * time.Second,
				ConcurrentWorkers: 1,
			}

			// Create discovery service with test logger
			slogLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelError,
			}))
			testLogger := logger.NewPlainStyledLogger(slogLogger)

			service := NewModelDiscoveryService(client, endpointRepo, modelRegistry, config, testLogger)

			// Discover models for the endpoint
			err := service.DiscoverEndpoint(ctx, tt.endpoint)
			require.NoError(t, err)

			// Verify the correct models were registered
			registered, exists := registeredModels[tt.endpoint.URLString]
			require.True(t, exists, "Models should be registered for endpoint")

			// Extract model names
			var registeredNames []string
			for _, model := range registered {
				registeredNames = append(registeredNames, model.Name)
			}

			// Verify filtering worked correctly
			assert.ElementsMatch(t, tt.expectedModels, registeredNames,
				"Registered models should match expected after filtering")
		})
