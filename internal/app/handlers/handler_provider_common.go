package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// createProviderProfile builds routing constraints for provider-specific requests.
// OpenAI providers are special - they accept traffic from any OpenAI-compatible
// endpoint to maximise compatibility across different implementations.
func (a *Application) createProviderProfile(providerType string) *domain.RequestProfile {
	profile := domain.NewRequestProfile("")

	// Profile factory has the complete prefix->provider mappings from YAML configs
	if a.profileFactory != nil {
		providerType = a.profileFactory.NormalizeProviderName(providerType)
	} else {
		// Test scenarios may not have full profile loading
		providerType = NormaliseProviderType(providerType)
	}

	// OpenAI routing is inclusive - any OpenAI-compatible endpoint can serve these requests
	if providerType == constants.ProviderTypeOpenAI || providerType == constants.ProviderTypeOpenAICompat {
		profile.AddSupportedProfile(domain.ProfileOpenAICompatible)

		// Dynamically include all providers that advertise OpenAI compatibility
		if a.profileFactory != nil {
			for _, profileName := range a.profileFactory.GetAvailableProfiles() {
				if p, err := a.profileFactory.GetProfile(profileName); err == nil {
					if config := p.GetConfig(); config != nil && config.API.OpenAICompatible {
						profile.AddSupportedProfile(profileName)
					}
				}
			}
		} else {
			// Test/static fallback: sourced from staticSupportedProviders
			// (handler_common.go), the single source of truth shared with
			// isProviderSupported's fallback and getStaticProviders
			// (server_routes.go).
			for _, p := range staticSupportedProviders {
				profile.AddSupportedProfile(p)
			}
		}
	} else {
		// Non-OpenAI providers only route to their specific backend type - this
		// is a hard boundary, not a compatibility hint, so filterEndpointsByProfile
		// must not silently widen to other types if none match (issue: cross-type
		// leak during a same-type outage).
		profile.AddSupportedProfile(providerType)
		profile.FailClosedOnNoMatch = true
	}

	return profile
}

// providerProxyHandler implements provider-scoped load balancing.
// Unlike the main proxy which balances across all endpoints, this constrains
// traffic to a specific provider type (e.g., only Ollama instances).
func (a *Application) providerProxyHandler(w http.ResponseWriter, r *http.Request) {
	providerType, _, ok := extractProviderFromPath(r.URL.Path)
	if !ok {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}

	// extractProviderFromPath already normalises, but we still need to verify
	// it's a provider we actually have endpoints for
	if !a.isProviderSupported(providerType) {
		http.Error(w, fmt.Sprintf("Unknown provider type: %s", providerType), http.StatusBadRequest)
		return
	}

	pr := a.initializeProxyRequest(r)

	// Provider type must flow through the entire request lifecycle to ensure
	// consistent routing decisions
	ctx := r.Context()
	ctx = context.WithValue(ctx, constants.ContextProviderTypeKey, providerType)

	// The proxy needs to know which prefix to strip before forwarding.
	// We must use the RAW (pre-normalisation) path segment as the strip prefix so
	// that alias spellings like /olla/lmstudio/ strip correctly. Using the
	// normalised name (lm-studio) would produce a non-matching prefix and forward
	// the full /olla/lmstudio/... path to the backend, causing a 404.
	providerPrefix := getRawProviderPrefix(r.URL.Path)
	ctx = context.WithValue(ctx, constants.ContextRoutePrefixKey, providerPrefix)
	r = r.WithContext(ctx)

	ctx, r = a.setupRequestContext(r, pr.stats)
	a.analyzeRequest(ctx, r, pr)

	// Sticky session key must be computed after analyzeRequest so the model name
	// is populated; inject into context before endpoint selection so the sticky
	// balancer wrapper observes the key during Select(). Mirrors proxyHandler.
	if a.Config.Proxy.StickySessions.Enabled {
		ctx, r, _ = a.injectStickyKey(ctx, r, a.canonicalModelName(pr.model))
	}

	endpoints, err := a.getProviderEndpoints(ctx, providerType, pr)
	if err != nil {
		a.handleEndpointError(w, pr, err)
		return
	}

	a.dispatchToEndpoints(ctx, w, r, pr, endpoints, providerType)
}

// getProviderEndpoints returns only endpoints matching the requested provider type.
// This ensures Ollama requests only go to Ollama backends, etc.
func (a *Application) getProviderEndpoints(ctx context.Context, providerType string, pr *proxyRequest) ([]*domain.Endpoint, error) {
	endpoints, err := a.discoveryService.GetHealthyEndpoints(ctx)
	if err != nil {
		pr.requestLogger.Error("Failed to get healthy endpoints", "error", err)
		return nil, fmt.Errorf("no healthy endpoints available: %w", err)
	}

	// We reuse the existing RequestProfile filtering logic by creating a profile
	// that only accepts the requested provider type
	providerProfile := a.createProviderProfile(providerType)
	providerProfile.Path = pr.targetPath

