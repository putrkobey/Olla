package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Endpoint id consistency across payloads: the same endpoint must report the
// SAME id across every /internal/status* payload. The dashboard keys model click-through
// rows on models[].endpoint_ids and matches them against endpoints[].id, so
// any drift breaks the click-through silently. Each status handler builds its
// own ID map from its own snapshot, so the only way they agree is if every
// handler derives IDs via the same buildEndpointIDs precompute over the same
// endpoint set.

// TestEndpointIDs_AcrossPayloadsConsistent drives a single Application
// fixture (one endpoint set, one model map) through BOTH the
// /internal/status/endpoints handler and the /internal/status/models
// handler, and asserts the ID for each endpoint is byte-for-byte identical
// across the two payloads. The fixture deliberately uses query-bearing URLs
// so the disambiguator path is exercised - a query-only-differing pair is
// exactly the case where the old per-handler raw-URL hash could have drifted
// if the two handlers had ever diverged in derivation.
func TestEndpointIDs_AcrossPayloadsConsistent(t *testing.T) {
	t.Parallel()

	const urlA = "http://host:11434/v1?api_key=alpha"
	const urlB = "http://host:11434/v1?api_key=beta"

	endpoints := []*domain.Endpoint{
		{Name: "node-a", Type: "ollama", URLString: urlA, Status: domain.StatusHealthy, Priority: 5},
		{Name: "node-b", Type: "ollama", URLString: urlB, Status: domain.StatusHealthy, Priority: 5},
	}
	// Both endpoints host the same model so ModelSummary.EndpointIDs is
	// populated for that model with one entry per endpoint.
	registry := &mockStatusModelRegistry{
		endpointModels: map[string]*domain.EndpointModels{
			urlA: {Models: []*domain.ModelInfo{{Name: "llama3", LastSeen: time.Now()}}},
			urlB: {Models: []*domain.ModelInfo{{Name: "llama3", LastSeen: time.Now()}}},
		},
	}
	app := &Application{
		repository:     &mockStatusEndpointRepository{endpoints: endpoints},
		statsCollector: &mockStatusStatsCollector{},
		modelRegistry:  registry,
		Config:         &config.Config{Proxy: config.ProxyConfig{Engine: "olla", Profile: "balanced", LoadBalancer: "round_robin"}},
		StartTime:      time.Now(),
	}

	// Drive /internal/status/endpoints.
	endpointsReq := httptest.NewRequest(http.MethodGet, "/internal/status/endpoints", nil)
