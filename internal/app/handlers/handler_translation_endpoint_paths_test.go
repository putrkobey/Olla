package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/theme"
)

// These tests drive the real proxy engines against backends that only serve the paths their
// real counterparts do. The mock proxies used elsewhere never build a URL, so a request sent
// to the wrong path went unnoticed: DMR and Lemonade 404 on the translator's fixed
// /v1/chat/completions.

const (
	dmrChatPath      = "/engines/v1/chat/completions"
	lemonadeChatPath = "/api/v1/chat/completions"
	rootChatPath     = "/v1/chat/completions"
)

// pathBackend answers only the paths in serves and records every path it was asked for.
type pathBackend struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

const (
	pathBackendChatJSON = `{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`
	pathBackendMessage  = `{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`
	pathBackendSSE      = "data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hi\"}}]}\n\n" +
		"data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n" +
		"data: [DONE]\n\n"
)

func newPathBackend(t *testing.T, serves ...string) *pathBackend {
	t.Helper()
	b := &pathBackend{}
	b.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		b.mu.Lock()
		b.hits = append(b.hits, r.URL.Path)
		b.mu.Unlock()
		for _, p := range serves {
			if r.URL.Path != p {
				continue
			}
			switch {
			case strings.HasSuffix(p, "/v1/messages"):
				w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
				_, _ = w.Write([]byte(pathBackendMessage))
			case strings.Contains(string(body), `"stream":true`):
				w.Header().Set(constants.HeaderContentType, "text/event-stream")
				_, _ = w.Write([]byte(pathBackendSSE))
			default:
				w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
				_, _ = w.Write([]byte(pathBackendChatJSON))
			}
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(b.Close)
	return b
}

func (b *pathBackend) paths() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.hits...)
}

// closedBackendURL returns an address nothing listens on, so connecting is refused and the
// engines fail over. A port freed by a closed httptest server can be reissued to another
// parallel test, so a privileged port is used instead.
func closedBackendURL(t *testing.T) string {
	t.Helper()
	return "http://127.0.0.1:1"
}

func liveEndpoint(t *testing.T, name, typ, rawURL string, priority int) *domain.Endpoint {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &domain.Endpoint{Name: name, Type: typ, URL: u, URLString: rawURL, Priority: priority, Status: domain.StatusHealthy}
}

type translationPathApp struct {
	app     *Application
	handler http.Handler
	trans   *anthropic.Translator
	log     *warnCounter
}

// newTranslationPathApp wires the shipped profiles, a real PathInspector and the chosen real
// engine. hosts maps model name to the endpoints that host it; strict picks the model routing
// strategy (strict rejects, optimistic falls back to every healthy endpoint).
func newTranslationPathApp(t *testing.T, engine string, strict, passthrough bool, endpoints []*domain.Endpoint, hosts map[string][]*domain.Endpoint) *translationPathApp {
	t.Helper()

	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)

	appLog := newWarnCounter()
	pathInspector := inspector.NewPathInspector(factory, appLog)
	chain := inspector.NewChain(appLog)
	chain.AddInspector(pathInspector)
	bodyInspector, err := inspector.NewBodyInspector(appLog)
	require.NoError(t, err)
	chain.AddInspector(bodyInspector)

	trans, err := anthropic.NewTranslator(appLog, config.AnthropicTranslatorConfig{PassthroughEnabled: passthrough})
	require.NoError(t, err)

	endpointsForModel := make(map[string][]string, len(hosts))
	for model, eps := range hosts {
		for _, ep := range eps {
			endpointsForModel[model] = append(endpointsForModel[model], ep.URLString)
		}
	}

	engineLog := logger.NewStyledLogger(slog.New(slog.NewTextHandler(io.Discard, nil)), &theme.Theme{}, false)
	collector := stats.NewCollector(engineLog)
	selector, err := balancer.NewFactory(collector).Create(balancer.DefaultBalancerPriority)
	require.NoError(t, err)
	discovery := &mockDiscoveryServiceWithFunc{getHealthyEndpointsFunc: func(context.Context) ([]*domain.Endpoint, error) {
		return endpoints, nil
	}}

	app := &Application{
		Config:           &config.Config{},
		logger:           appLog,
		rejectLog:        newRejectLogs(appLog, nil),
		inspectorChain:   chain,
		pathInspector:    pathInspector,
		profileFactory:   factory,
		profileLookup:    factory,
		discoveryService: discovery,
		modelRegistry:    &mockSimpleModelRegistry{strict: strict, endpointsForModel: endpointsForModel},
		statsCollector:   collector,
	}

	switch engine {
	case "sherpa":
		cfg := &sherpa.Configuration{}
		cfg.ResponseTimeout, cfg.ReadTimeout, cfg.StreamBufferSize = 5*time.Second, 5*time.Second, 8192
		svc, serr := sherpa.NewService(discovery, selector, cfg, collector, nil, engineLog)
		require.NoError(t, serr)
		app.proxyService = svc
	case "olla":
		cfg := &olla.Configuration{}
		cfg.ResponseTimeout, cfg.ReadTimeout, cfg.StreamBufferSize = 5*time.Second, 5*time.Second, 8192
		svc, serr := olla.NewService(discovery, selector, cfg, collector, nil, engineLog)
		require.NoError(t, serr)
		t.Cleanup(svc.Cleanup)
		app.proxyService = svc
	default:
		t.Fatalf("unknown engine %q", engine)
	}

	return &translationPathApp{app: app, handler: app.translationHandler(trans), trans: trans, log: appLog}
}

func (f *translationPathApp) post(t *testing.T, model string, stream bool) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 16,
		"stream":     stream,
		"messages":   []map[string]any{{"role": "user", "content": "hi"}},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/olla/anthropic/v1/messages", bytes.NewReader(b))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	req.RemoteAddr = "203.0.113.7:4000"
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, req)
	return w
}

// requireTranslatedOK checks the response is a translated Anthropic message (not passthrough).
func requireTranslatedOK(t *testing.T, w *httptest.ResponseRecorder, stream bool) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	assert.Empty(t, w.Header().Get(constants.HeaderXOllaMode), "must be translation, not passthrough")
	if stream {
		assert.Contains(t, w.Body.String(), "message_stop")
		return
	}
	var msg map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &msg))
	assert.Equal(t, "message", msg["type"])
}

var (
	translationEngines = []string{"sherpa", "olla"}
	translationModes   = []struct {
		name   string
		strict bool
	}{{"strict", true}, {"optimistic", false}}
	translationStreaming = []struct {
		name   string
		stream bool
	}{{"non-streaming", false}, {"streaming", true}}
)

// A mixed fleet: each model lives on one backend, and translation must reach that backend
