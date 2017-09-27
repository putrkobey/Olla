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
