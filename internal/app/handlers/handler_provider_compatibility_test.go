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
