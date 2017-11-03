package discovery

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestModelDiscoveryServiceStartStop(t *testing.T) {
	client := &mockDiscoveryClient{}
	endpointRepo := &mockEndpointRepository{}
	modelRegistry := domain.ModelRegistry(&mockModelRegistry{})
	config := DiscoveryConfig{
		Interval:          100 * time.Millisecond,
		Timeout:           5 * time.Second,
		ConcurrentWorkers: 2,
		RetryAttempts:     3,
		RetryBackoff:      time.Millisecond,
	}

	service := NewModelDiscoveryService(client, endpointRepo, modelRegistry, config, createTestLogger())

	ctx := context.Background()
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	if !service.isRunning.Load() {
		t.Errorf("Service should be running after start")
	}

	err = service.Start(ctx)
	if err == nil {
		t.Errorf("Expected error when starting already running service")
	}

	err = service.Stop(ctx)
	if err != nil {
		t.Errorf("Failed to stop service: %v", err)
	}

	// Give time for goroutines to clean up
	time.Sleep(10 * time.Millisecond)

	if service.isRunning.Load() {
		t.Errorf("Service should be stopped after stop")
	}

	// we should protected against double stops
	err = service.Stop(ctx)
	if err != nil {
		t.Errorf("Double stop should not error: %v", err)
	}
}

func TestDiscoverAll(t *testing.T) {
	tests := []struct {
		name                string
		healthyEndpoints    []*domain.Endpoint
		repositoryError     error
		discoveryErrors     map[string]error
		expectedDiscoveries int
		expectedError       bool
	}{
		{
			name: "successful discovery from multiple endpoints",
			healthyEndpoints: []*domain.Endpoint{
				createMockEndpoint("http://localhost:11434", "ollama-1"),
				createMockEndpoint("http://localhost:1234", "lm-studio-1"),
			},
			discoveryErrors:     map[string]error{},
			expectedDiscoveries: 2,
			expectedError:       false,
		},
		{
			name:                "no healthy endpoints",
			healthyEndpoints:    []*domain.Endpoint{},
			discoveryErrors:     map[string]error{},
			expectedDiscoveries: 0,
			expectedError:       false,
		},
		{
			name:            "repository error",
			repositoryError: errors.New("repository error"),
			expectedError:   true,
		},
		{
			name: "partial discovery failures",
			healthyEndpoints: []*domain.Endpoint{
				createMockEndpoint("http://localhost:11434", "ollama-1"),
				createMockEndpoint("http://localhost:1234", "lm-studio-1"),
			},
			discoveryErrors: map[string]error{
				"http://localhost:11434": errors.New("discovery failed"),
			},
			expectedDiscoveries: 2, // Both attempted, one fails
			expectedError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockDiscoveryClient{
				discoveryErrors: tt.discoveryErrors,
			}
			endpointRepo := &mockEndpointRepository{
				healthyEndpoints: tt.healthyEndpoints,
				getHealthyError:  tt.repositoryError,
			}
			modelRegistry := &mockModelRegistry{}
			config := DiscoveryConfig{
				Timeout:           5 * time.Second,
				ConcurrentWorkers: 2,
			}

			service := NewModelDiscoveryService(client, endpointRepo, modelRegistry, config, createTestLogger())

			err := service.DiscoverAll(context.Background())

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if client.discoverCallCount != tt.expectedDiscoveries {
				t.Errorf("Expected %d discovery calls, got %d", tt.expectedDiscoveries, client.discoverCallCount)
			}
		})
	}
}

func TestDiscoverEndpoint(t *testing.T) {
	tests := []struct {
		name                 string
		endpoint             *domain.Endpoint
		discoveryError       error
		registryError        error
		expectedModelCount   int
		expectedError        bool
		expectRegistryCalled bool
	}{
		{
			name:                 "successful discovery and registration",
			endpoint:             createMockEndpoint("http://localhost:11434", "ollama-1"),
			discoveryError:       nil,
			registryError:        nil,
			expectedModelCount:   2,
			expectedError:        false,
			expectRegistryCalled: true,
		},
		{
			name:                 "discovery failure",
			endpoint:             createMockEndpoint("http://localhost:11434", "ollama-1"),
			discoveryError:       errors.New("network error"),
			expectedError:        true,
			expectRegistryCalled: false,
		},
		{
			name:                 "registry failure",
			endpoint:             createMockEndpoint("http://localhost:11434", "ollama-1"),
			discoveryError:       nil,
			registryError:        errors.New("registry error"),
			expectedError:        true,
			expectRegistryCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoveryErrors := make(map[string]error)
			if tt.discoveryError != nil {
				discoveryErrors[tt.endpoint.URLString] = tt.discoveryError
			}

			client := &mockDiscoveryClient{
				discoveryErrors: discoveryErrors,
			}
			endpointRepo := &mockEndpointRepository{}
			modelRegistry := &mockModelRegistry{
				registeredModels: make([]*domain.ModelInfo, 0),
				registerError:    tt.registryError,
			}
			config := DiscoveryConfig{
				Timeout: 5 * time.Second,
			}

			service := NewModelDiscoveryService(client, endpointRepo, modelRegistry, config, createTestLogger())

			err := service.DiscoverEndpoint(context.Background(), tt.endpoint)

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectRegistryCalled && modelRegistry.registerCallCount == 0 {
				t.Errorf("Expected registry to be called but it wasn't")
			}
			if !tt.expectRegistryCalled && modelRegistry.registerCallCount > 0 {
				t.Errorf("Expected registry not to be called but it was")
			}

			// check that the right number of models registered considering it seems successful
			if !tt.expectedError && tt.expectRegistryCalled && len(modelRegistry.registeredModels) != tt.expectedModelCount {
				t.Errorf("Expected %d models registered, got %d (call count: %d, actual models: %v)",
					tt.expectedModelCount, len(modelRegistry.registeredModels), modelRegistry.registerCallCount, modelRegistry.registeredModels)
			}
		})
	}
}

func TestEndpointDisabling(t *testing.T) {
	tests := []struct {
		name                   string
		discoveryError         error
		expectedDisabled       bool
		consecutiveFailures    int
		expectFailureIncrement bool
	}{
		{
			name:                   "non-recoverable error disables immediately",
			discoveryError:         NewDiscoveryError("http://test", "ollama", "http_status", 401, 0, errors.New("unauthorised")),
			expectedDisabled:       true,
			expectFailureIncrement: false,
		},
		{
			name:                   "recoverable error increments failure count",
			discoveryError:         &NetworkError{URL: "http://test", Err: errors.New("network error")},
			expectedDisabled:       false,
			expectFailureIncrement: true,
		},
		{
			name:                   "success resets failure count",
			discoveryError:         nil,
			expectedDisabled:       false,
			expectFailureIncrement: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := createMockEndpoint("http://localhost:11434", "test-endpoint")

			client := &mockDiscoveryClient{
				discoveryErrors: map[string]error{
					endpoint.URLString: tt.discoveryError,
				},
			}
			endpointRepo := &mockEndpointRepository{}
			modelRegistry := &mockModelRegistry{
				registeredModels: make([]*domain.ModelInfo, 0),
			}
			config := DiscoveryConfig{Timeout: 5 * time.Second}

			service := NewModelDiscoveryService(client, endpointRepo, modelRegistry, config, createTestLogger())

			// Simulate previous failures if needed
			for range tt.consecutiveFailures {
				service.incrementFailureCount(endpoint.URLString)
			}

			_ = service.DiscoverEndpoint(context.Background(), endpoint)

			isDisabled := service.isEndpointDisabled(endpoint.URLString)
			if isDisabled != tt.expectedDisabled {
				t.Errorf("Expected endpoint disabled state to be %v, got %v", tt.expectedDisabled, isDisabled)
			}

