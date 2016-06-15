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

