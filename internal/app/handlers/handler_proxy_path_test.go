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
