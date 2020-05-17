package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestProxyPathStripping tests the critical path stripping behavior for both proxy implementations
// This is critical for edge deployments where millions of requests rely on correct routing
//
// NOTE: This test documents the ACTUAL behavior, including known issues that exist in production.
// Some behaviors are not ideal but changing them could break existing clients.
func TestProxyPathStripping(t *testing.T) {
	testCases := []struct {
		name          string
		proxyPrefix   string
		requestPath   string
		contextPrefix string // If set, overrides config prefix via context
		expectedPath  string
		description   string
	}{
		// Normal cases
		{
			name:         "standard_prefix_stripping",
			proxyPrefix:  "/olla",
			requestPath:  "/olla/api/chat",
			expectedPath: "/api/chat",
			description:  "Standard prefix removal",
		},
		{
			name:         "provider_prefix_stripping",
			proxyPrefix:  "/olla/proxy",
			requestPath:  "/olla/proxy/v1/chat/completions",
			expectedPath: "/v1/chat/completions",
			description:  "Provider proxy prefix removal",
		},
		{
			name:         "no_prefix_in_path",
			proxyPrefix:  "/olla",
			requestPath:  "/api/chat",
			expectedPath: "/api/chat",
			description:  "Path without prefix remains unchanged",
		},

		// Edge cases
		{
			name:         "empty_path_after_strip",
			proxyPrefix:  "/olla",
			requestPath:  "/olla",
			expectedPath: "/",
			description:  "Empty path becomes root",
		},
		{
			name:         "prefix_only_partial_match",
			proxyPrefix:  "/olla",
			requestPath:  "/ollama/api/chat",
			expectedPath: "/ma/api/chat", // This is the ACTUAL behavior (a bug!)
			description:  "Partial prefix match incorrectly stripped (BUG)",
		},
		{
			name:         "double_slash_handling",
			proxyPrefix:  "/olla",
			requestPath:  "/olla//api/chat",
			expectedPath: "//api/chat",
			description:  "Double slashes preserved after prefix",
		},
		{
			name:         "trailing_slash_prefix",
			proxyPrefix:  "/olla/",
			requestPath:  "/olla/api/chat",
			expectedPath: "/api/chat",
			description:  "Trailing slash in prefix handled correctly",
		},

		// Security cases - document actual behavior
		{
			name:         "path_traversal_attempt",
