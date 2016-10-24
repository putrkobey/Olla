package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/theme"
)

// The per-request "Request completed" line is the operator's record of a proxied request,
// so it must say what status the client got and a backend 5xx must not read as a clean
// completion. These tests drive real engines over every route kind and read the log back.

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

type logLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
	// kept loose: slog encodes ints as JSON numbers, and "status" is a string on completed lines
	HTTPStatus *float64 `json:"http_status"`
	Status     any      `json:"status"`
}

func (b *lockedBuffer) lines(t *testing.T, msg string) []logLine {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []logLine
	for _, raw := range strings.Split(b.buf.String(), "\n") {
		if raw == "" {
			continue
		}
		var l logLine
		require.NoError(t, json.Unmarshal([]byte(raw), &l), raw)
		if l.Msg == msg {
			out = append(out, l)
		}
	}
	return out
}

type statusRoute struct {
	name    string
	handler string // proxy, provider, anthropic
	path    string
	// passthrough only matters for the anthropic route
	passthrough bool
}

func statusRoutes() []statusRoute {
	return []statusRoute{
		{name: "proxy", handler: "proxy", path: "/olla/proxy/v1/chat/completions"},
		{name: "provider", handler: "provider", path: "/olla/ollama/v1/chat/completions"},
		{name: "anthropic-passthrough", handler: "anthropic", path: "/olla/anthropic/v1/messages", passthrough: true},
		{name: "anthropic-translation", handler: "anthropic", path: "/olla/anthropic/v1/messages"},
	}
}

type statusHarness struct {
	fleet *tagFleet
	logs  *lockedBuffer
	route statusRoute
}

func newStatusHarness(t *testing.T, engine string, route statusRoute, backendURL string) *statusHarness {
	t.Helper()

	fleet := newTagFleet(t, tagStrategies[0].cfg, map[string]string{"ollama-1": backendURL}, false, route.passthrough)

	logs := &lockedBuffer{}
	log := logger.NewStyledLogger(slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})), &theme.Theme{}, false)
	collector := stats.NewCollector(log)
	selector, err := balancer.NewFactory(collector).Create(balancer.DefaultBalancerPriority)
	require.NoError(t, err)

	fleet.app.logger = log
	fleet.app.rejectLog = newRejectLogs(log, nil)
	fleet.app.statsCollector = collector
	fleet.app.discoveryService = &mockDiscoveryServiceWithHealthy{endpoints: fleet.endpoints[:1]}

	switch engine {
	case "sherpa":
		cfg := &sherpa.Configuration{}
		cfg.ResponseTimeout, cfg.ReadTimeout, cfg.StreamBufferSize = 5*time.Second, 5*time.Second, 8192
		svc, serr := sherpa.NewService(fleet.app.discoveryService, selector, cfg, collector, nil, log)
		require.NoError(t, serr)
		fleet.app.proxyService = svc
	default:
		cfg := &olla.Configuration{}
		cfg.ResponseTimeout, cfg.ReadTimeout, cfg.StreamBufferSize = 5*time.Second, 5*time.Second, 8192
		svc, serr := olla.NewService(fleet.app.discoveryService, selector, cfg, collector, nil, log)
		require.NoError(t, serr)
		t.Cleanup(svc.Cleanup)
		fleet.app.proxyService = svc
	}
	return &statusHarness{fleet: fleet, logs: logs, route: route}
}

func (h *statusHarness) post(model string, stream bool) *httptest.ResponseRecorder {
	body := fmt.Sprintf(`{"model":%q,"stream":%t,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`, model, stream)
	req := httptest.NewRequest(http.MethodPost, h.route.path, strings.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w := httptest.NewRecorder()

	switch h.route.handler {
	case "anthropic":
		h.fleet.app.translationHandler(h.fleet.trans).ServeHTTP(w, req)
	case "proxy":
		req = req.WithContext(context.WithValue(req.Context(), constants.ContextRoutePrefixKey, "/olla/proxy/"))
		h.fleet.app.proxyHandler(w, req)
	default:
		h.fleet.app.providerProxyHandler(w, req)
	}
	return w
}

// statusBackend answers every request with the given status. When stream is set it writes
// an event-stream in two flushed chunks, so the status is committed at the first write.
func statusBackend(t *testing.T, status int, stream bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stream {
			w.Header().Set(constants.HeaderContentType, "text/event-stream")
			w.WriteHeader(status)
			for _, chunk := range []string{
