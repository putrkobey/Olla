package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestFilterEndpointsByProfile_WithModel(t *testing.T) {
	// Create mock logger
	styledLog := &mockStyledLogger{}

	// Create test endpoints
	endpoint1, _ := url.Parse("http://localhost:11434")
	endpoint2, _ := url.Parse("http://localhost:11435")
	endpoint3, _ := url.Parse("http://localhost:11436")

	endpoints := []*domain.Endpoint{
		{
			Name:      "endpoint1",
			URL:       endpoint1,
			URLString: "http://localhost:11434",
			Type:      domain.ProfileOllama,
		},
		{
			Name:      "endpoint2",
			URL:       endpoint2,
			URLString: "http://localhost:11435",
			Type:      domain.ProfileOllama,
		},
		{
			Name:      "endpoint3",
			URL:       endpoint3,
			URLString: "http://localhost:11436",
			Type:      domain.ProfileLmStudio,
		},
	}

	// Mock model registry
	modelRegistry := &mockSimpleModelRegistry{
		endpointsForModel: map[string][]string{
			"llama3.1:8b": {"http://localhost:11434", "http://localhost:11435"},
			"codellama":   {"http://localhost:11436"},
		},
	}

	app := &Application{
		modelRegistry: modelRegistry,
		logger:        styledLog,
	}

	t.Run("filters by model when available", func(t *testing.T) {
		profile := &domain.RequestProfile{
			Path:        "/v1/chat/completions",
			ModelName:   "llama3.1:8b",
			SupportedBy: []string{domain.ProfileOllama, domain.ProfileLmStudio},
		}

		filtered := app.filterEndpointsByProfile(endpoints, profile, styledLog)

		// Should only return endpoint1 and endpoint2
		assert.Len(t, filtered, 2)
		assert.Contains(t, filtered, endpoints[0])
		assert.Contains(t, filtered, endpoints[1])
		assert.NotContains(t, filtered, endpoints[2])
	})

	t.Run("returns all compatible when model not found", func(t *testing.T) {
		profile := &domain.RequestProfile{
			Path:        "/v1/chat/completions",
			ModelName:   "gpt-4",
			SupportedBy: []string{domain.ProfileOllama, domain.ProfileLmStudio},
		}

		filtered := app.filterEndpointsByProfile(endpoints, profile, styledLog)

		// Should return all compatible endpoints
		assert.Len(t, filtered, 3)
	})

	t.Run("no model filtering when model not specified", func(t *testing.T) {
		profile := &domain.RequestProfile{
			Path:        "/v1/chat/completions",
			ModelName:   "", // No model specified
			SupportedBy: []string{domain.ProfileOllama, domain.ProfileLmStudio},
		}

		filtered := app.filterEndpointsByProfile(endpoints, profile, styledLog)

		// Should return all compatible endpoints
		assert.Len(t, filtered, 3)
	})
}

func TestBodyInspectorIntegration(t *testing.T) {
	// Create mock logger
	styledLog := &mockStyledLogger{}

	// Create profile factory
	profileFactory, err := profile.NewFactoryWithDefaults()
	require.NoError(t, err)

	// Create inspector chain
	inspectorFactory := inspector.NewFactory(profileFactory, styledLog)
	chain := inspectorFactory.CreateChain()

	// Add inspectors
	pathInspector := inspectorFactory.CreatePathInspector()
	bodyInspector, err := inspectorFactory.CreateBodyInspector()
	if err != nil {
		t.Fatalf("Failed to create body inspector: %v", err)
	}
	chain.AddInspector(pathInspector)
	chain.AddInspector(bodyInspector)

	tests := []struct {
		name          string
		path          string
		body          string
		expectedModel string
	}{
		{
			name:          "Extracts model from OpenAI format",
			path:          "/v1/chat/completions",
			body:          `{"model": "gpt-4", "messages": []}`,
			expectedModel: "gpt-4",
		},
		{
			name:          "Extracts model from Ollama format",
