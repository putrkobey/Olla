package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	"github.com/putrkobey/Olla/internal/logger"
)

// Verifies that proxyHandler strips the route prefix from r.URL.Path before
// forwarding to the backend. Without this, backends receive the full
// /olla/proxy/... path which they cannot route.
func TestProxyPathStripping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		requestPath  string
		routePrefix  string
		expectedPath string
	}{
		{
			name:         "strips_olla_proxy_prefix_from_openai_path",
			requestPath:  "/olla/proxy/v1/chat/completions",
			routePrefix:  "/olla/proxy/",
			expectedPath: "/v1/chat/completions",
		},
		{
			name:         "strips_olla_proxy_prefix_from_ollama_path",
			requestPath:  "/olla/proxy/api/generate",
			routePrefix:  "/olla/proxy/",
			expectedPath: "/api/generate",
		},
		{
			name:         "no_route_prefix_leaves_path_unchanged",
			requestPath:  "/v1/chat/completions",
			routePrefix:  "", // no prefix injected
			expectedPath: "/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockLogger := &mockStyledLogger{}

			// Capture the path that the proxy service actually receives
			var capturedPath string
			proxyService := &mockProxyService{
				proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, logger logger.StyledLogger) error {
					capturedPath = r.URL.Path
					w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte(`{"ok":true}`))
					return err
				},
			}

			app := &Application{
				logger:           mockLogger,
				proxyService:     proxyService,
				discoveryService: &mockDiscoveryServiceForTranslation{},
				inspectorChain:   inspector.NewChain(mockLogger),
				profileFactory:   &mockProfileFactory{},
				Config: &config.Config{
