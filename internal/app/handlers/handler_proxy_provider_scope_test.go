package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestFilterEndpointsByProfile_FailClosedOnNoMatch is the unit-level guard
// for the provider-scoped cross-type leak: filterEndpointsByProfile's
// zero-compatible-endpoints fallback must not widen to all endpoints when the
// profile's SupportedBy represents a hard provider-type boundary
// (FailClosedOnNoMatch, set by createProviderProfile for a non-OpenAI
// provider-scoped route). A vllm-scoped request with only an
// openai-compatible endpoint healthy must get nothing back, not the
// wrong-type endpoint.
func TestFilterEndpointsByProfile_FailClosedOnNoMatch(t *testing.T) {
	styledLog := &mockStyledLogger{}
	app := &Application{logger: styledLog}

	openaiURL, _ := url.Parse("http://localhost:8000")
	endpoints := []*domain.Endpoint{{
		Name:      "openai-compat-1",
		URL:       openaiURL,
		URLString: openaiURL.String(),
		Type:      domain.ProfileOpenAICompatible,
		Status:    domain.StatusHealthy,
	}}

	profile := &domain.RequestProfile{
		SupportedBy:         []string{"vllm"},
		FailClosedOnNoMatch: true,
	}

	filtered := app.filterEndpointsByProfile(endpoints, profile, styledLog)
	if len(filtered) != 0 {
		t.Fatalf("expected zero endpoints when a provider-scoped profile has no type match, got %d: %v", len(filtered), filtered)
	}
}

// TestFilterEndpointsByProfile_WidensWhenNotFailClosed pins the inverse: a
// profile that does NOT set FailClosedOnNoMatch (the OpenAI-inclusive
// profile, and the unscoped /olla/proxy/ route's inspector-derived profile)
