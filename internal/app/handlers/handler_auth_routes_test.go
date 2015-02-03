package handlers

// TestAuthAcrossProxyRoutes proves that every proxy-bearing route handler passes
// an auth-configured endpoint to the proxy service unchanged.
//
// Background: issue #139 revealed that providerProxyHandler was not wired through
// the same middleware path as proxyHandler, so cross-cutting concerns (sticky
// sessions in that case, auth injection in general) could silently be skipped.
//
// This test catches the next #139-style regression: if a new handler family is
// added that bypasses executeProxyRequest (and therefore bypasses the CopyHeaders
// call that injects endpoint credentials), this test will fail because the proxy
// service will never be invoked with the auth endpoint.
//
// What this test does NOT cover:
//   - Whether CopyHeaders correctly injects the auth header (covered by
//     internal/adapter/proxy/core/common_auth_test.go)
//   - End-to-end network delivery of the header to the backend (out of scope for
//     handler-layer tests; that lives in the proxy engine tests)
//
// Route families covered:
//   - /olla/proxy/          → proxyHandler
//   - /olla/ollama/         → providerProxyHandler (representative of all provider routes)
//   - /olla/anthropic/v1/messages → translationHandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	styledlogger "github.com/putrkobey/Olla/internal/logger"
)

// authEndpoint returns an endpoint configured with bearer auth. We use this as
// the only entry in the discovery service so every proxy route must route through it.
func authEndpoint(t *testing.T, providerType string) *domain.Endpoint {
	t.Helper()
	u, _ := url.Parse("http://localhost:11434")
	return &domain.Endpoint{
		Name:            "auth-endpoint",
		URL:             u,
		URLString:       u.String(),
		Type:            providerType,
		Status:          domain.StatusHealthy,
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: "Bearer test-backend-secret",
	}
}

// authCapturingProxyService records the endpoints passed in by the handler so we
// can assert the auth-configured endpoint was forwarded unmodified.
type authCapturingProxyService struct {
	capturedEndpoints []*domain.Endpoint
	capturedCtx       context.Context
}

func (s *authCapturingProxyService) ProxyRequestToEndpoints(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	endpoints []*domain.Endpoint,
	stats *ports.RequestStats,
	_ styledlogger.StyledLogger,
) error {
	s.capturedEndpoints = endpoints
	s.capturedCtx = ctx
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
	return nil
}

func (s *authCapturingProxyService) ProxyRequest(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	stats *ports.RequestStats,
	_ styledlogger.StyledLogger,
) error {
	return nil
}

func (s *authCapturingProxyService) GetStats(ctx context.Context) (ports.ProxyStats, error) {
	return ports.ProxyStats{}, nil
}
func (s *authCapturingProxyService) UpdateConfig(c ports.ProxyConfiguration) {}

// discoveryWithAuth returns a discovery service that yields a single
// auth-configured endpoint of the given provider type.
func discoveryWithAuth(ep *domain.Endpoint) *mockDiscoveryServiceWithHealthy {
	return &mockDiscoveryServiceWithHealthy{endpoints: []*domain.Endpoint{ep}}
}

// minimalApp builds an Application wired for auth route tests.
func minimalApp(t *testing.T, capture *authCapturingProxyService, ds ports.DiscoveryService, log *mockStyledLogger) *Application {
	t.Helper()

	return &Application{
		logger:           log,
		proxyService:     capture,
		discoveryService: ds,
		inspectorChain:   inspector.NewChain(log),
		profileFactory: &mockProfileFactory{
			validProfiles: map[string]bool{
				"ollama":    true,
				"openai":    true,
				"lmstudio":  true,
				"lm-studio": true,
				"vllm":      true,
			},
		},
		statsCollector: &mockStatsCollector{},
		repository:     &mockEndpointRepository{},
		Config: &config.Config{
			Server: config.ServerConfig{RateLimits: config.ServerRateLimits{}},
		},
		StartTime: time.Now(),
	}
}

// TestAuthAcrossProxyRoutes_ProxyHandler verifies proxyHandler forwards the
// auth-configured endpoint to the proxy service. This route has always worked
// correctly; the test serves as a baseline for the parameterised coverage.
func TestAuthAcrossProxyRoutes_ProxyHandler(t *testing.T) {
	t.Parallel()

	ep := authEndpoint(t, "openai")
	capture := &authCapturingProxyService{}
	mockLog := &mockStyledLogger{}
	app := minimalApp(t, capture, discoveryWithAuth(ep), mockLog)

	body := `{"model":"llama3","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/olla/proxy/v1/chat/completions",
		strings.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)

	// Inject the route prefix the router would normally set.
	ctx := context.WithValue(req.Context(), constants.ContextRoutePrefixKey, "/olla/proxy/")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.proxyHandler(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "proxyHandler must complete successfully")
	assertAuthEndpointReached(t, "proxyHandler", capture, ep)
}

// TestAuthAcrossProxyRoutes_ProviderProxyHandler verifies providerProxyHandler (the
// route used by /olla/ollama/, /olla/openai/, etc.) also forwards the auth endpoint.
// This is the handler that was wired incorrectly in issue #139. Had this test
// existed then, the missing sticky-session wiring would have been caught first.
func TestAuthAcrossProxyRoutes_ProviderProxyHandler(t *testing.T) {
	t.Parallel()

	ep := authEndpoint(t, "ollama")
	capture := &authCapturingProxyService{}
	mockLog := &mockStyledLogger{}
	app := minimalApp(t, capture, discoveryWithAuth(ep), mockLog)

