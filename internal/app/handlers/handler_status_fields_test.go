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
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Asserts the additive dashboard fields render on /internal/status/endpoints
// with the agreed types and zero-traffic conventions: min/max latency are
// plain int64 zeros, avg_latency_ms is omitted when there is no traffic, and
// absolute timestamps are RFC3339 alongside the relative strings.
func TestEndpointsStatusHandler_AdditiveFields(t *testing.T) {
	const trafficURL = "http://localhost:11434"
	const idleURL = "http://localhost:8080"

	endpoints := []*domain.Endpoint{
		{
			Name:          "busy",
			Type:          "ollama",
			URLString:     trafficURL,
			Status:        domain.StatusHealthy,
			Priority:      1,
			LastChecked:   time.Now().Add(-15 * time.Second),
			NextCheckTime: time.Now().Add(45 * time.Second),
		},
		{
			Name:          "idle",
			Type:          "openai",
			URLString:     idleURL,
			Status:        domain.StatusHealthy,
			Priority:      2,
			LastChecked:   time.Now().Add(-15 * time.Second),
			NextCheckTime: time.Now().Add(45 * time.Second),
		},
	}

	stats := &mockStatusStatsCollector{
		endpointStats: map[string]ports.EndpointStats{
			trafficURL: {
				TotalRequests:      10,
				SuccessfulRequests: 9,
				MinLatency:         30,
				MaxLatency:         400,
				AverageLatency:     150,
			},
		},
		connectionStats: map[string]int64{
			trafficURL: 2,
			idleURL:    0,
		},
	}

	app := &Application{
		repository:     &mockStatusEndpointRepository{endpoints: endpoints},
		statsCollector: stats,
		modelRegistry:  &mockStatusModelRegistry{},
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/status/endpoints", nil)
	w := httptest.NewRecorder()
	app.endpointsStatusHandler(w, req)

	var resp EndpointStatusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
