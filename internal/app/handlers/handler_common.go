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

	// normalise the provider name to canonical form
	provider = NormaliseProviderType(provider)

	return provider, remainingPath, true
}

// modifyRequestPath updates the request path for backend routing.
// preserves the original path in context for logging/debugging
func (a *Application) modifyRequestPath(r *http.Request, newPath string) *http.Request {
	// store original path for reference
	ctx := context.WithValue(r.Context(), constants.ContextOriginalPathKey, r.URL.Path)

	// update path
	r.URL.Path = newPath
	if r.URL.RawPath != "" {
		r.URL.RawPath = newPath
	}

	return r.WithContext(ctx)
}

// staticSupportedProviders is the single source of truth for provider types
// recognised when the profile factory isn't available - test contexts and the
// hardcoded route registration in registerStaticProviderRoutes. Production
// always goes through the profile factory (loaded from config/profiles/*.yaml);
// this list only matters for that fallback path.
//
// Three call sites used to each keep their own copy of this list - getStaticProviders
// (server_routes.go), isProviderSupported's fallback (below) and createProviderProfile's
// OpenAI-compatible fallback (handler_provider_common.go) - and they drifted:
// LMDeploy, SGLang, DMR and OMLX were inconsistently present across the three.
var staticSupportedProviders = []string{
	constants.ProviderTypeLemonade,
	constants.ProviderTypeOllama,
	constants.ProviderTypeLMStudio,
	constants.ProviderTypeOpenAI,
	constants.ProviderTypeLMDeploy,
	constants.ProviderTypeSGLang,
	constants.ProviderTypeVLLM,
	constants.ProviderTypeOMLX,
	constants.ProviderTypeDockerMR,
	constants.ProviderTypeTextEmbeddingsInference,
}

