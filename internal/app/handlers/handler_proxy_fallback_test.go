package handlers

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/theme"
)

// TestModelFallbackWhenEndpointGoesOffline tests the specific scenario reported by the user:
// When an endpoint with a model goes offline, requests should fall back to other healthy endpoints
func TestModelFallbackWhenEndpointGoesOffline(t *testing.T) {
	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockLogger := logger.NewStyledLogger(testLogger, &theme.Theme{}, false)

	// Create test endpoints - note localOllama is offline (not in healthy list)
	macOllama := &domain.Endpoint{
		Name:      "mac-ollama",
		URLString: "http://192.168.0.144:11434",
		Type:      "ollama",
		Status:    domain.StatusHealthy,
		Priority:  100,
	}

	// Mock registry that knows local-ollama has the model (even though it's offline)
	mockRegistry := &mockModelRegistryForFallback{
		endpointsForModel: map[string][]string{
			"phi3.5:latest": {"http://localhost:11434"}, // Only the offline endpoint has it
		},
	}

	app := &Application{
		logger:        mockLogger,
		modelRegistry: mockRegistry,
	}

	// Create request profile for phi3.5:latest
	profile := &domain.RequestProfile{
		ModelName:   "phi3.5:latest",
		Path:        "/api/chat",
		SupportedBy: []string{"ollama"},
	}

	// Test filtering - only healthy endpoints passed in
	healthyEndpoints := []*domain.Endpoint{macOllama}
	filtered := app.filterEndpointsByProfile(healthyEndpoints, profile, mockLogger)

	// Should return mac-ollama even though it doesn't have the model in registry
	// because local-ollama (which has the model) is offline
	if len(filtered) != 1 {
		t.Fatalf("Expected 1 endpoint after filtering, got %d", len(filtered))
	}

	if filtered[0].Name != "mac-ollama" {
