package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// NormaliseProviderType handles the various ways users might specify a provider
// (e.g., lmstudio, lm-studio, lm_studio all map to "lm-studio").
// This ensures consistent internal representation regardless of user input.
func NormaliseProviderType(provider string) string {
	normalised := strings.ToLower(strings.ReplaceAll(provider, "_", "-"))

	// LM Studio is special - it has multiple common variations that all need
	// to map to the canonical form for profile lookups to work correctly
	switch normalised {
	case constants.ProviderPrefixLMStudio1, constants.ProviderPrefixLMStudio2:
		return constants.ProviderTypeLMStudio
	default:
		return normalised
	}
}

// extractProviderFromPath parses URLs like /olla/ollama/api/chat to extract
// the provider type and remaining path for backend routing.
func extractProviderFromPath(path string) (provider string, remainingPath string, ok bool) {
	if !strings.HasPrefix(path, constants.DefaultOllaProxyPathPrefix) {
		return "", "", false
	}

	// remove /olla/ prefix
	withoutPrefix := strings.TrimPrefix(path, constants.DefaultOllaProxyPathPrefix)

	// find next slash to extract provider
	slashIdx := strings.Index(withoutPrefix, constants.DefaultPathPrefix)
	if slashIdx == -1 {
		// path like /olla/ollama with no trailing part
		return withoutPrefix, constants.DefaultPathPrefix, true
	}

	provider = withoutPrefix[:slashIdx]
	remainingPath = withoutPrefix[slashIdx:]
