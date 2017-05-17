package handlers

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/router"
)

func psBackend(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ps" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func psEndpoint(name, typ string, srv *httptest.Server, status domain.EndpointStatus) *domain.Endpoint {
	u, _ := url.Parse(srv.URL)
	return &domain.Endpoint{Name: name, Type: typ, URL: u, URLString: srv.URL, Status: status}
}

func runPS(t *testing.T, ctx context.Context, eps ...*domain.Endpoint) *httptest.ResponseRecorder {
	t.Helper()
	app := &Application{logger: &mockStyledLogger{}, repository: &mockRepository{endpoints: eps}}
	req := httptest.NewRequest(http.MethodGet, "/olla/ollama/api/ps", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	app.ollamaRunningModelsHandler(w, req)
	return w
}

type psResponse struct {
	Models []map[string]any `json:"models"`
}

func TestOllamaRunningModels_MergesPerInstance(t *testing.T) {
	t.Parallel()

	a := psBackend(t, 200, `{"models":[{"name":"llama3:8b","size_vram":10},{"name":"qwen3:8b"}]}`)
	b := psBackend(t, 200, `{"models":[{"name":"llama3:8b","size_vram":20}]}`)
	other := psBackend(t, 200, `{"models":[{"name":"not-ollama"}]}`)
	down := psBackend(t, 200, `{"models":[{"name":"unhealthy"}]}`)

	w := runPS(t, context.Background(),
		psEndpoint("a", "ollama", a, domain.StatusHealthy),
		psEndpoint("b", "ollama", b, domain.StatusHealthy),
		psEndpoint("v", "vllm", other, domain.StatusHealthy),
		psEndpoint("d", "ollama", down, domain.StatusUnhealthy),
	)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Empty(t, w.Header().Get(headerOllaPartial))

	var got psResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Models, 3, "the same model on two hosts stays as two entries")
	assert.Equal(t, "a", got.Models[0][ollamaPSEndpointField])
	assert.Equal(t, "llama3:8b", got.Models[0]["name"])
	assert.EqualValues(t, 10, got.Models[0]["size_vram"], "fields pass through untouched")
	assert.Equal(t, "b", got.Models[2][ollamaPSEndpointField])
}

func TestOllamaRunningModels_SkipsFailingInstances(t *testing.T) {
	t.Parallel()

	good := psBackend(t, 200, `{"models":[{"name":"llama3:8b"}]}`)
	for name, bad := range map[string]*httptest.Server{
		"status":    psBackend(t, 500, `boom`),
		"malformed": psBackend(t, 200, `{"models":`),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			w := runPS(t, context.Background(),
				psEndpoint("good", "ollama", good, domain.StatusHealthy),
				psEndpoint("bad", "ollama", bad, domain.StatusHealthy))

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "1 of 2 endpoints failed", w.Header().Get(headerOllaPartial))
			var got psResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			require.Len(t, got.Models, 1)
			assert.Equal(t, "good", got.Models[0][ollamaPSEndpointField])
		})
	}
}

func TestOllamaRunningModels_TimeoutDoesNotHoldTheAnswer(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); hang.Close() })
	good := psBackend(t, 200, `{"models":[{"name":"llama3:8b"}]}`)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	w := runPS(t, ctx,
		psEndpoint("good", "ollama", good, domain.StatusHealthy),
		psEndpoint("hang", "ollama", hang, domain.StatusHealthy))

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "1 of 2 endpoints failed", w.Header().Get(headerOllaPartial))
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestOllamaRunningModels_AllFailingIs502_NoInstancesIsEmpty(t *testing.T) {
	t.Parallel()

	bad := psBackend(t, 500, `boom`)
	w := runPS(t, context.Background(), psEndpoint("bad", "ollama", bad, domain.StatusHealthy))
	assert.Equal(t, http.StatusBadGateway, w.Code)

	w = runPS(t, context.Background(), psEndpoint("v", "vllm", bad, domain.StatusHealthy))
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"models":[]}`, w.Body.String())
}

func TestOllamaRunningModels_SendsEndpointAuthAndRefusesRedirects(t *testing.T) {
	t.Parallel()

	var gotAuth string
	authed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	t.Cleanup(authed.Close)
	ep := psEndpoint("authed", "ollama", authed, domain.StatusHealthy)
	ep.AuthHeaderName, ep.AuthHeaderValue = "Authorization", "Bearer tok"
	require.Equal(t, http.StatusOK, runPS(t, context.Background(), ep).Code)
	assert.Equal(t, "Bearer tok", gotAuth)

	target := psBackend(t, 200, `{"models":[{"name":"elsewhere"}]}`)
	redirector := httptest.NewServer(http.RedirectHandler(target.URL+"/api/ps", http.StatusFound))
	t.Cleanup(redirector.Close)
	w := runPS(t, context.Background(), psEndpoint("r", "ollama", redirector, domain.StatusHealthy))
