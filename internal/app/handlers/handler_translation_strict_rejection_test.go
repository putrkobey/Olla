package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// TestTranslationHandler_StrictRejection_DecisionAwareStatusAndHeaders covers fix 3
// from the #191 follow-up review: the translation route's zero-endpoint branch used to
// always return a hardcoded 404 via writeTranslatorError, ignoring pr.profile.RoutingDecision
// entirely. That flattened a strict "model only on unhealthy endpoints" 503 to 404 and never
// set X-Olla-Routing-* headers, unlike the equivalent proxy-route rejection
// (writeNoRoutableEndpoints). This proves the translation route now honours the recorded
// decision's status code and reason, sets the routing headers, and still returns an
// Anthropic-shaped JSON error body (the Anthropic route must never fall back to the
// plain-text writeNoRoutableEndpoints format).
func TestTranslationHandler_StrictRejection_DecisionAwareStatusAndHeaders(t *testing.T) {
	t.Parallel()

	styledLog := &mockStyledLogger{}

	// Real inspector chain (path + body) so profile.ModelName is populated from the
	// request body exactly as it is in production, matching the #191 proxy-route tests.
	chain := inspector.NewChain(styledLog)
	bodyInspector, err := inspector.NewBodyInspector(styledLog)
	require.NoError(t, err)
	chain.AddInspector(bodyInspector)

	trans, err := anthropic.NewTranslator(styledLog, config.AnthropicTranslatorConfig{})
	require.NoError(t, err)

	// Strict registry: the requested model exists nowhere in the fleet, so
	// GetRoutableEndpointsForModel rejects with a genuine strict decision
	// (action "rejected", reason model_not_found, status 404).
	modelRegistry := &mockSimpleModelRegistry{
		endpointsForModel: map[string][]string{},
		strict:            true,
	}

	app := &Application{
		Config:           &config.Config{},
		logger:           styledLog,
		inspectorChain:   chain,
		discoveryService: &mockDiscoveryServiceForTranslation{},
		modelRegistry:    modelRegistry,
		statsCollector:   &mockStatsCollector{},
		proxyService: &mockProxyService{
			proxyFunc: nil, // must never be reached
		},
	}

	body := `{"model":"claude-not-registered","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/olla/anthropic/v1/messages", strings.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w := httptest.NewRecorder()

	app.translationHandler(trans).ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "must use the decision's status code, not a hardcoded default")
	assert.Equal(t, "strict", w.Header().Get(constants.HeaderXOllaRoutingStrategy))
	assert.Equal(t, "rejected", w.Header().Get(constants.HeaderXOllaRoutingDecision))
	assert.Equal(t, "model_not_found", w.Header().Get(constants.HeaderXOllaRoutingReason))
