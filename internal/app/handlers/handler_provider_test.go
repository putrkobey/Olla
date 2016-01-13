package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// mockDiscoveryService for testing
type mockDiscoveryService struct{}

func (m *mockDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (m *mockDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (m *mockDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	return nil
}

func (m *mockDiscoveryService) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	return nil
}

// createTestApplication creates a minimal Application for testing
func createTestApplication(t *testing.T) *Application {
	logger := &mockStyledLogger{}
	profileFactory := &mockProfileFactory{
		validProfiles: map[string]bool{
			"ollama":    true,
			"lmstudio":  true,
			"lm-studio": true,
			"openai":    true,
			"vllm":      true,
		},
	}

	// Create minimal config
	cfg := &config.Config{
		Server: config.ServerConfig{
			RateLimits: config.ServerRateLimits{},
		},
	}

	// Create empty inspector chain
	inspectorChain := inspector.NewChain(logger)

	return &Application{
		Config:           cfg,
		logger:           logger,
		discoveryService: &mockDiscoveryService{},
		profileFactory:   profileFactory,
		inspectorChain:   inspectorChain,
		StartTime:        time.Now(),
	}
}

// TestProviderRouting tests basic provider routing functionality by calling the actual handler
func TestProviderRouting(t *testing.T) {
	tests := []struct {
		name               string
		url                string
		method             string
		expectedStatusCode int
		expectedError      string
	}{
		{
			name:               "Invalid provider path - too short",
			url:                "/olla",
			method:             "POST",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      "Invalid path format",
		},
		{
			name:               "Unknown provider type",
			url:                "/olla/unknown/api/test",
			method:             "POST",
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      "Unknown provider type: unknown",
		},
		{
			name:               "Valid Ollama provider",
			url:                "/olla/ollama/api/generate",
			method:             "POST",
			expectedStatusCode: http.StatusNotFound,
			expectedError:      "No ollama endpoints available",
		},
		{
			name:               "Valid LM Studio provider",
			url:                "/olla/lmstudio/v1/chat/completions",
			method:             "POST",
			expectedStatusCode: http.StatusNotFound,
			expectedError:      "No lm-studio endpoints available",
		},
		{
			name:               "Valid OpenAI provider",
			url:                "/olla/openai/v1/models",
			method:             "POST",
			expectedStatusCode: http.StatusNotFound,
			expectedError:      "No openai endpoints available",
		},
		{
			name:               "Valid vLLM provider",
			url:                "/olla/vllm/generate",
			method:             "POST",
			expectedStatusCode: http.StatusNotFound,
			expectedError:      "No vllm endpoints available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := createTestApplication(t)
			req := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			// Call the actual handler method
			app.providerProxyHandler(w, req)

			resp := w.Result()
