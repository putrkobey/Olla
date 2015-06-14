package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestProviderCompatibility tests the provider compatibility logic using RequestProfile
func TestProviderCompatibility(t *testing.T) {
	// Create a minimal Application for testing
	app := &Application{}

	tests := []struct {
		name         string
		endpointType string
		providerType string
		compatible   bool
	}{
		// OpenAI provider should accept all OpenAI-compatible endpoints
		{
			name:         "openai provider accepts ollama",
			endpointType: "ollama",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts lmstudio",
			endpointType: "lmstudio",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts lm-studio",
			endpointType: "lm-studio",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts openai",
			endpointType: "openai",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts openai-compatible",
			endpointType: "openai-compatible",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts vllm",
			endpointType: "vllm",
			providerType: "openai",
			compatible:   true,
		},
		{
			name:         "openai provider accepts lemonade",
			endpointType: "lemonade",
			providerType: "openai",
			compatible:   true,
		},
		// Ollama provider should only accept ollama endpoints
		{
			name:         "ollama provider accepts ollama",
			endpointType: "ollama",
			providerType: "ollama",
			compatible:   true,
		},
		{
			name:         "ollama provider rejects lmstudio",
			endpointType: "lmstudio",
			providerType: "ollama",
			compatible:   false,
		},
		{
			name:         "ollama provider rejects openai",
			endpointType: "openai",
			providerType: "ollama",
			compatible:   false,
		},
		// LM Studio provider should only accept lmstudio endpoints
		{
			name:         "lmstudio provider accepts lmstudio",
			endpointType: "lmstudio",
			providerType: "lmstudio",
			compatible:   true,
		},
		{
			name:         "lmstudio provider accepts lm-studio",
			endpointType: "lm-studio",
			providerType: "lmstudio",
			compatible:   true,
		},
		{
			name:         "lmstudio provider rejects ollama",
			endpointType: "ollama",
			providerType: "lmstudio",
			compatible:   false,
		},
		{
			name:         "lmstudio provider rejects openai",
			endpointType: "openai",
			providerType: "lmstudio",
			compatible:   false,
		},
		// vLLM provider should only accept vllm endpoints
		{
			name:         "vllm provider accepts vllm",
			endpointType: "vllm",
			providerType: "vllm",
			compatible:   true,
		},
		{
			name:         "vllm provider rejects ollama",
			endpointType: "ollama",
			providerType: "vllm",
			compatible:   false,
		},
		// Lemonade provider should only accept lemonade endpoints
		{
			name:         "lemonade provider accepts lemonade",
			endpointType: "lemonade",
			providerType: "lemonade",
			compatible:   true,
		},
		{
			name:         "lemonade provider rejects ollama",
			endpointType: "ollama",
			providerType: "lemonade",
			compatible:   false,
		},
		{
			name:         "lemonade provider rejects vllm",
			endpointType: "vllm",
			providerType: "lemonade",
			compatible:   false,
		},
		// oMLX provider should only accept omlx endpoints
		{
			name:         "omlx provider accepts omlx",
			endpointType: "omlx",
			providerType: "omlx",
			compatible:   true,
		},
		{
			name:         "omlx provider rejects ollama",
			endpointType: "ollama",
			providerType: "omlx",
			compatible:   false,
		},
		{
			name:         "omlx provider rejects vllm",
			endpointType: "vllm",
			providerType: "omlx",
			compatible:   false,
		},
		// Unknown provider should reject everything
		{
			name:         "unknown provider rejects ollama",
			endpointType: "ollama",
			providerType: "unknown",
			compatible:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// create a profile for the provider type
			profile := app.createProviderProfile(tt.providerType)

			// Normalise endpoint type to match what would be in the system
			normalizedEndpoint := NormaliseProviderType(tt.endpointType)

			result := profile.IsCompatibleWith(normalizedEndpoint)
			assert.Equal(t, tt.compatible, result, "Expected %v for %s -> %s", tt.compatible, tt.endpointType, tt.providerType)
		})
	}
}

// TestProviderCompatibility_RealFactory exercises createProviderProfile against
// a real profile factory loaded from YAML. The nil-factory path (used by the
// test above) masked issue #148 because it hardcodes "openai" into SupportedBy
// directly, bypassing the IsCompatibleWith alias logic entirely.
//
// This test represents the production code path: an endpoint configured
// type:"openai" must appear in GET /olla/openai/v1/models results.
func TestProviderCompatibility_RealFactory(t *testing.T) {
	t.Parallel()

	factory, err := profile.NewFactory("../../../config/profiles")
	if err != nil {
		t.Fatalf("failed to load profile factory from YAML: %v", err)
	}

	app := &Application{profileFactory: factory}

	tests := []struct {
		name         string
		providerType string
		endpointType string
		wantCompat   bool
	}{
		// The production failure: an endpoint with type:"openai" must be included
		// when the client calls GET /olla/openai/v1/models.
		{
			name:         "openai provider accepts openai-typed endpoint (issue #148)",
