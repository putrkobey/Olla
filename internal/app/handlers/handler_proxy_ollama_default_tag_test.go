package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/theme"
)

// Ollama lists a default-tag pull as "name:latest" while clients ask for "name".
// Nothing normalised that, so the bare name was a 404 on every route. These tests
// walk the route table so a path that bypasses the registry rule shows up here.

const (
	tagOllamaTags = `{"models":[
		{"name":"all-minilm:latest","capabilities":["embedding"]},
		{"name":"mistral-small3.2:latest","capabilities":["completion"]},
		{"name":"qwen2.5:0.5b","capabilities":["completion"]}]}`
	tagVLLMModels = `{"data":[{"id":"Qwen/Qwen3-32B","object":"model"},{"id":"BAAI/bge-m3","object":"model"}]}`

	tagBareEmbed = "all-minilm"
	tagBareChat  = "mistral-small3.2"
)

var tagStrategies = []struct {
	name string
	cfg  config.ModelRoutingStrategy
}{
	{"strict", config.ModelRoutingStrategy{Type: "strict"}},
	{"optimistic-compatible-only", config.ModelRoutingStrategy{Type: "optimistic", Options: config.ModelRoutingStrategyOptions{FallbackBehavior: constants.FallbackBehaviorCompatibleOnly}}},
	{"optimistic-all", config.ModelRoutingStrategy{Type: "optimistic", Options: config.ModelRoutingStrategyOptions{FallbackBehavior: constants.FallbackBehaviorAll}}},
}

// tagFleet builds an application over a real unified registry holding one Ollama
// and one vLLM host, parsed through the shipped profiles like production discovery.
type tagFleet struct {
	app       *Application
	registry  *registry.UnifiedMemoryModelRegistry
	endpoints []*domain.Endpoint
	trans     *anthropic.Translator
}

func newTagFleet(t *testing.T, strategy config.ModelRoutingStrategy, urls map[string]string, sticky bool, passthrough bool) *tagFleet {
	t.Helper()

	log := &mockStyledLogger{}
	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)

	pathInspector := inspector.NewPathInspector(factory, log)
	chain := inspector.NewChain(log)
	chain.AddInspector(pathInspector)
	bodyInspector, err := inspector.NewBodyInspector(log)
	require.NoError(t, err)
	chain.AddInspector(bodyInspector)

	trans, err := anthropic.NewTranslator(log, config.AnthropicTranslatorConfig{PassthroughEnabled: passthrough})
	require.NoError(t, err)

	cfg := strategy
	unified := registry.NewUnifiedMemoryModelRegistry(log, nil, &cfg, nil)

	ctx := context.Background()
	var endpoints []*domain.Endpoint
	var registered []*domain.ModelInfo
	for _, spec := range []struct{ name, typ, models string }{
		{"ollama-1", domain.ProfileOllama, tagOllamaTags},
		{"vllm-1", domain.ProfileVLLM, tagVLLMModels},
	} {
		raw := urls[spec.name]
		if raw == "" {
			raw = "http://" + spec.name + ".test:8080"
		}
		u, perr := url.Parse(raw)
		require.NoError(t, perr)
		ep := &domain.Endpoint{Name: spec.name, URL: u, URLString: u.String(), Type: spec.typ, Status: domain.StatusHealthy}
		endpoints = append(endpoints, ep)

		prof, perr := factory.GetProfile(spec.typ)
		require.NoError(t, perr)
		models, perr := prof.ParseModelsResponse([]byte(spec.models))
		require.NoError(t, perr)
		require.NoError(t, unified.RegisterModelsWithEndpoint(ctx, ep, models))
		registered = append(registered, models...)
	}
	// Unification is asynchronous: wait until every native name resolves.
	require.Eventually(t, func() bool {
		for _, m := range registered {
