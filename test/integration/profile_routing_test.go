package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
)

// TestProfileRoutingPrefixes verifies that the profile factory correctly
// validates provider types using the routing prefixes defined in profiles
func TestProfileRoutingPrefixes(t *testing.T) {
	// Create profile factory which loads profiles with routing prefixes
	profileFactory, err := profile.NewFactoryWithDefaults()
	require.NoError(t, err)

	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		// Direct profile names
		{"ollama profile", "ollama", true},
		{"lm-studio profile", "lm-studio", true},
		{"openai-compatible profile", "openai-compatible", true},

		// Routing prefixes for lm-studio (from lmstudio.yaml)
		{"lmstudio prefix", "lmstudio", true},
		{"lm_studio prefix", "lm_studio", true},

		// Routing prefix for openai (from openai-compatible.yaml, which owns the "openai" prefix)
		{"openai prefix", "openai", true},

		// Auto profile
		{"auto profile", "auto", true},

		// Invalid providers
		{"unknown provider", "unknown", false},
		{"empty provider", "", false},

		// Case sensitivity test
		{"OLLAMA uppercase", "OLLAMA", false},
		{"LmStudio mixed", "LmStudio", false},
	}
