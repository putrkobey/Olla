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
			if u, uerr := unified.GetUnifiedModel(ctx, m.Name); uerr != nil || u == nil {
				return false
			}
		}
		return true
	}, 5*time.Second, 5*time.Millisecond)

	appCfg := &config.Config{}
	if sticky {
		appCfg.Proxy.StickySessions = config.StickySessionConfig{Enabled: true, KeySources: []string{"session_header"}, MaxSessions: 100, IdleTTLSeconds: 60, PrefixHashBytes: 512}
	}

	app := &Application{
		Config:           appCfg,
		logger:           log,
		inspectorChain:   chain,
		profileFactory:   factory,
		profileLookup:    factory,
		discoveryService: &mockDiscoveryServiceWithHealthy{endpoints: endpoints},
		modelRegistry:    unified,
		repository:       &mockRepository{endpoints: endpoints},
		statsCollector:   &mockStatsCollector{},
		StartTime:        time.Now(),
	}
	return &tagFleet{app: app, registry: unified, endpoints: endpoints, trans: trans}
}

// seen is what the proxy engine was handed for one request.
type seen struct {
	ctx       context.Context
	decision  *domain.ModelRoutingDecision
	body      string
	endpoints []string
}

func (f *tagFleet) capture() *seen {
	got := &seen{}
	f.app.proxyService = &mockProxyService{proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, eps []*domain.Endpoint, st *ports.RequestStats, _ logger.StyledLogger) error {
		got.ctx = ctx
		got.decision = st.RoutingDecision
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			got.body = string(b)
		}
		for _, ep := range eps {
			got.endpoints = append(got.endpoints, ep.Name)
		}
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
		return nil
	}}
	return got
}

type tagRoute struct {
	name string
	path string
	// bare is the model's untagged name; the tagged spelling is bare + ":latest"
	bare string
	// handler picks which entry point serves the route
	handler string
}

func tagRoutes() []tagRoute {
	var routes []tagRoute
	add := func(prefix, handler string, embed, chat []string) {
		for _, p := range embed {
			routes = append(routes, tagRoute{prefix + p, prefix + p, tagBareEmbed, handler})
		}
		for _, p := range chat {
			routes = append(routes, tagRoute{prefix + p, prefix + p, tagBareChat, handler})
		}
	}
	add("/olla/proxy", "proxy", []string{"/v1/embeddings", "/api/embed", "/api/embeddings"}, []string{"/v1/chat/completions", "/api/chat", "/api/generate"})
	add("/olla/ollama", "provider", []string{"/v1/embeddings", "/api/embed", "/api/embeddings"}, []string{"/v1/chat/completions", "/api/chat", "/api/generate"})
	// The OpenAI-compatible prefixes also serve Ollama hosts
	add("/olla/openai", "provider", []string{"/v1/embeddings"}, []string{"/v1/chat/completions"})
	add("/olla/openai-compatible", "provider", []string{"/v1/embeddings"}, []string{"/v1/chat/completions"})
	routes = append(routes,
		tagRoute{"anthropic messages", "/olla/anthropic/v1/messages", tagBareChat, "anthropic"},
		tagRoute{"model detail", "/olla/models/", tagBareEmbed, "detail"},
	)
	return routes
}

func (f *tagFleet) do(t *testing.T, route tagRoute, model string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()

	switch route.handler {
	case "detail":
		mux := http.NewServeMux()
		mux.HandleFunc("GET /olla/models/", f.app.unifiedModelByAliasHandler)
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route.path+url.PathEscape(model), nil))
		return w
	case "anthropic":
		body := `{"model":"` + model + `","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
		req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(body))
		req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
		f.app.translationHandler(f.trans).ServeHTTP(w, req)
		return w
	}

	body := `{"model":"` + model + `","input":"hi","prompt":"hi","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	if route.handler == "proxy" {
		req = req.WithContext(context.WithValue(req.Context(), constants.ContextRoutePrefixKey, "/olla/proxy/"))
		f.app.proxyHandler(w, req)
	} else {
		f.app.providerProxyHandler(w, req)
	}
