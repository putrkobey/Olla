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
