package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestMetricsHandler_BasicFunctionality(t *testing.T) {
	t.Parallel()

	endpoints := []*domain.Endpoint{
		{
			Name:      "test-endpoint-healthy",
			Type:      "ollama",
			URLString: "http://localhost:11434",
			Status:    domain.StatusHealthy,
			Priority:  100,
		},
		{
			Name:      "test-endpoint-unhealthy",
			Type:      "openai",
			URLString: "http://localhost:8080",
			Status:    domain.StatusUnhealthy,
			Priority:  50,
		},
	}

	stats := &mockStatusStatsCollector{
		endpointStats: map[string]ports.EndpointStats{
			"http://localhost:11434": {
				TotalRequests:      120,
				SuccessfulRequests: 118,
				FailedRequests:     2,
				TotalBytes:         4096,
				AverageLatency:     125,
			},
		},
		proxyStats: ports.ProxyStats{
			TotalRequests:      120,
			SuccessfulRequests: 118,
			FailedRequests:     2,
			AverageLatency:     125,
		},
	}

	app := &Application{
		repository:     &mockStatusEndpointRepository{endpoints: endpoints},
