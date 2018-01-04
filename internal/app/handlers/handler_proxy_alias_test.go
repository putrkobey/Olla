package handlers

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestResolveAliasEndpoints_ResolvesToCorrectEndpoints(t *testing.T) {
	styledLog := &mockStyledLogger{}

	endpoint1URL, _ := url.Parse("http://ollama:11434")
	endpoint2URL, _ := url.Parse("http://lmstudio:1234")
	endpoint3URL, _ := url.Parse("http://llamacpp:8080")

	candidates := []*domain.Endpoint{
		{
			Name:      "ollama",
			URL:       endpoint1URL,
			URLString: "http://ollama:11434",
			Type:      domain.ProfileOllama,
		},
		{
			Name:      "lmstudio",
			URL:       endpoint2URL,
			URLString: "http://lmstudio:1234",
			Type:      domain.ProfileLmStudio,
		},
		{
			Name:      "llamacpp",
			URL:       endpoint3URL,
			URLString: "http://llamacpp:8080",
			Type:      domain.ProfileLlamaCpp,
		},
	}

	modelRegistry := &mockSimpleModelRegistry{
		endpointsForModel: map[string][]string{
			"gpt-oss:120b":     {"http://ollama:11434"},
			"gpt-oss-120b-MLX": {"http://lmstudio:1234"},
			"some-other-model": {"http://llamacpp:8080"},
		},
	}

	aliases := map[string][]string{
		"gpt-oss-120b": {"gpt-oss:120b", "gpt-oss-120b-MLX", "gguf_gpt_oss_120b.gguf"},
	}
	aliasResolver := registry.NewAliasResolver(aliases, styledLog)

	app := &Application{
		modelRegistry: modelRegistry,
		aliasResolver: aliasResolver,
		logger:        styledLog,
	}

	profile := domain.NewRequestProfile("/v1/chat/completions")
	profile.ModelName = "gpt-oss-120b"
	profile.SupportedBy = []string{domain.ProfileOllama, domain.ProfileLmStudio}

	result := app.resolveAliasEndpoints(t.Context(), profile, candidates, styledLog)

	// Should return only ollama and lmstudio (not llamacpp, which doesn't have any aliased model)
	assert.Len(t, result, 2)
	assert.Contains(t, result, candidates[0]) // ollama
	assert.Contains(t, result, candidates[1]) // lmstudio

	// Verify the alias rewrite map was stored in the profile
	aliasMapRaw, ok := profile.InspectionMeta.Load(constants.ContextModelAliasMapKey)
	require.True(t, ok, "alias rewrite map should be stored in profile")

	aliasMap, ok := aliasMapRaw.(map[string]string)
	require.True(t, ok, "alias map should be map[string]string")

	assert.Equal(t, "gpt-oss:120b", aliasMap["http://ollama:11434"])
	assert.Equal(t, "gpt-oss-120b-MLX", aliasMap["http://lmstudio:1234"])
}

// TestResolveAliasEndpoints_NoMatchingEndpoints covers #191: when an alias resolves to
// no endpoints and the standard-routing fallback lookup also rejects (the target model
// exists nowhere in the fleet), the result must be empty. Returning candidates here would
// silently proxy the request to a backend that doesn't serve the requested model, defeating
// strict routing entirely. This is exactly the bug this test used to codify.
func TestResolveAliasEndpoints_NoMatchingEndpoints(t *testing.T) {
	styledLog := &mockStyledLogger{}

	endpoint1URL, _ := url.Parse("http://ollama:11434")
	candidates := []*domain.Endpoint{
		{
			Name:      "ollama",
			URL:       endpoint1URL,
			URLString: "http://ollama:11434",
			Type:      domain.ProfileOllama,
		},
	}

	// Registry has no models matching the alias, and strict mode means the standard-routing
	// fallback lookup rejects rather than handing back all candidates.
	modelRegistry := &mockSimpleModelRegistry{
		endpointsForModel: map[string][]string{},
		strict:            true,
	}

	aliases := map[string][]string{
		"nonexistent-alias": {"model-not-in-registry"},
	}
	aliasResolver := registry.NewAliasResolver(aliases, styledLog)

	app := &Application{
		modelRegistry: modelRegistry,
		aliasResolver: aliasResolver,
		logger:        styledLog,
	}

	profile := domain.NewRequestProfile("/v1/chat/completions")
	profile.ModelName = "nonexistent-alias"
	profile.SupportedBy = []string{domain.ProfileOllama}

	result := app.resolveAliasEndpoints(t.Context(), profile, candidates, styledLog)

	assert.Empty(t, result, "a rejected fallback lookup must fail fast, not return all candidates")

	require.NotNil(t, profile.RoutingDecision, "rejection decision should still be recorded for headers/metrics")
	assert.Equal(t, "rejected", profile.RoutingDecision.Action)
}

// mockBasicActionModelRegistry mimics MemoryModelRegistry's base GetRoutableEndpointsForModel
// (internal/adapter/registry/memory_registry.go), which reports rejections with action
// "no_model"/"no_healthy" rather than "rejected", but still sets a 4xx/5xx StatusCode.
// Used to prove the alias fail-fast short-circuit is keyed on status code, not the
// "rejected" action string (fix 1 from the #191 follow-up review).
type mockBasicActionModelRegistry struct {
	baseMockRegistry
}

func (m *mockBasicActionModelRegistry) GetRoutableEndpointsForModel(_ context.Context, _ string, _ []*domain.Endpoint) ([]*domain.Endpoint, *domain.ModelRoutingDecision, error) {
	return []*domain.Endpoint{}, &domain.ModelRoutingDecision{
		Strategy:   "basic",
		Action:     "no_model",
		Reason:     "Model not found in any endpoint",
		StatusCode: http.StatusNotFound,
	}, nil
}

// TestResolveAliasEndpoints_ShortCircuitsOnStatusCodeNotActionString covers fix 1 from
// the #191 follow-up review: the alias fail-fast short-circuit must key on
// decision.StatusCode (matching writeNoRoutableEndpoints' contract), not on
// decision.Action == "rejected". A registry that reports a 404 rejection under a
// different action name (as MemoryModelRegistry's base implementation does) must
// still short-circuit; keying on the action string alone would silently fall through
// and reintroduce the #191 bypass.
func TestResolveAliasEndpoints_ShortCircuitsOnStatusCodeNotActionString(t *testing.T) {
	styledLog := &mockStyledLogger{}

	endpoint1URL, _ := url.Parse("http://ollama:11434")
	candidates := []*domain.Endpoint{
		{
			Name:      "ollama",
			URL:       endpoint1URL,
			URLString: "http://ollama:11434",
			Type:      domain.ProfileOllama,
		},
	}

	// Alias resolves to nothing, so resolveAliasEndpoints falls through to the
	// standard-routing fallback lookup, which is where the short-circuit lives.
