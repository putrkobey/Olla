package handlers

import (
	"encoding/json"
	"hash"
	"hash/fnv"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/putrkobey/Olla/internal/core/ports"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/pkg/format"
)

type EndpointSummary struct {
	AvgLatencyMs    *int64     `json:"avg_latency_ms,omitempty"`
	NextCheckAt     *time.Time `json:"next_check_at,omitempty"`
	HealthCheckAt   *time.Time `json:"health_check_at,omitempty"`
	LastModelSyncAt *time.Time `json:"last_model_sync_at,omitempty"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Status          string     `json:"status"`
	LastModelSync   string     `json:"last_model_sync,omitempty"`
	HealthCheck     string     `json:"health_check"`
	ResponseTime    string     `json:"response_time,omitempty"`
	SuccessRate     string     `json:"success_rate"`
	Issues          string     `json:"issues,omitempty"`
	URL             string     `json:"url"`
	// ID is a stable, opaque identifier derived from the SANITISED endpoint
	// URL (scheme+host+port+path) via buildEndpointIDs, so credentials in the
	// query string cannot influence it and credential rotation leaves it
	// unchanged. Siblings that share a sanitised form get a positional
	// disambiguator assigned by sorted (secret-independent) sibling order.
	// See buildEndpointIDs and stableEndpointID.
	ID           string `json:"id"`
	Priority     int    `json:"priority"`
	ModelCount   int    `json:"model_count"`
	RequestCount int64  `json:"request_count"`
	// Additive dashboard fields, existing fields above are unchanged.
	// min/max latency follow the same plain-zero convention as RequestCount
	// for no-traffic endpoints; avg_latency_ms is a pointer so a no-traffic
	// endpoint omits the field rather than emitting a misleading 0.
	MinLatencyMs      int64 `json:"min_latency_ms"`
	MaxLatencyMs      int64 `json:"max_latency_ms"`
	ActiveConnections int64 `json:"active_connections"`
}

type EndpointStatusResponse struct {
	Timestamp     time.Time         `json:"timestamp"`
	Endpoints     []EndpointSummary `json:"endpoints"`
	TotalCount    int               `json:"total_count"`
	HealthyCount  int               `json:"healthy_count"`
	RoutableCount int               `json:"routable_count"`
}

const (
	healthyStatus = "healthy"
	// noTrafficSuccessRate is surfaced for both endpoint- and system-level
	// success_rate whenever there is no traffic to compute a real rate from.
	noTrafficSuccessRate = "N/A"
)

func (a *Application) endpointsStatusHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// get everyone all at once, then deal with it ted.
	allEndpoints, healthyEndpoints, routableEndpoints, err := a.getEndpointCounts(ctx)
	if err != nil {
		http.Error(w, "Failed to get endpoint data", http.StatusInternalServerError)
		return
	}

	endpointStats := a.statsCollector.GetEndpointStats()
	connectionStats := a.statsCollector.GetConnectionStats()
	modelMap, _ := a.modelRegistry.GetEndpointModelMap(ctx)
	// IDs are derived once from the full endpoint set so siblings that share
	// a sanitised URL get deterministic positional disambiguators, and every
	// payload (status/endpoints, status, status/models) sees the same ID for
	// the same endpoint from the same snapshot.
	endpointIDs := buildEndpointIDs(allEndpoints)
	summaries := make([]EndpointSummary, 0, len(allEndpoints))

	for _, endpoint := range allEndpoints {
		summary := a.buildEndpointSummaryOptimised(endpoint, endpointStats, connectionStats, modelMap, endpointIDs[endpoint.URLString])
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Priority != summaries[j].Priority {
			return summaries[i].Priority > summaries[j].Priority
		}
		// Within a priority band, healthy comes before anything else.
		iHealthy := summaries[i].Status == healthyStatus
		jHealthy := summaries[j].Status == healthyStatus
		if iHealthy != jHealthy {
			return iHealthy
		}
		// Tie-breaker for deterministic ordering across polls: the input
		// slice comes from map iteration, so without a final comparison
		// equal-priority same-health endpoints reorder between polls purely
		// from map-iteration randomisation. Name first, then ID for the
		// pathological case of two endpoints sharing a name. ID (not the
		// sanitised URL) because sanitisation strips query/fragment and two
		// distinct endpoints can share a display URL, which would leave the
		// comparison tied and ordering unstable again.
		if summaries[i].Name != summaries[j].Name {
			return summaries[i].Name < summaries[j].Name
		}
		return summaries[i].ID < summaries[j].ID
	})

	// create a response with minimal mallocs
	response := EndpointStatusResponse{
		Timestamp:     time.Now(),
		TotalCount:    len(allEndpoints),
		HealthyCount:  len(healthyEndpoints),
		RoutableCount: len(routableEndpoints),
		Endpoints:     summaries,
	}

	body, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	writeJSONWithETag(w, r, body, hashEndpointStatusResponse(&response))
}

func (a *Application) buildEndpointSummaryOptimised(endpoint *domain.Endpoint, statsMap map[string]ports.EndpointStats, connectionStats map[string]int64, modelMap map[string]*domain.EndpointModels, endpointID string) EndpointSummary {
	url := endpoint.URLString
	stats, hasStats := statsMap[url]
	models := modelMap[url]

	summary := EndpointSummary{
		Name:              endpoint.Name,
		Type:              endpoint.Type,
		Status:            endpoint.Status.String(),
		Priority:          endpoint.Priority,
		ID:                endpointID,
		URL:               sanitiseDisplayURL(url),
		ActiveConnections: connectionStats[url],
	}

	if models != nil {
		summary.ModelCount = len(models.Models)
		if !models.LastUpdated.IsZero() {
			summary.LastModelSync = format.TimeAgo(models.LastUpdated)
			latm := models.LastUpdated
			summary.LastModelSyncAt = &latm
		}
	}

	if !endpoint.LastChecked.IsZero() {
		summary.HealthCheck = format.TimeAgo(endpoint.LastChecked)
		lc := endpoint.LastChecked
		summary.HealthCheckAt = &lc
		if endpoint.LastLatency > 0 {
			summary.ResponseTime = format.Latency(endpoint.LastLatency.Milliseconds())
		}
	}

	if !endpoint.NextCheckTime.IsZero() {
		nc := endpoint.NextCheckTime
		summary.NextCheckAt = &nc
	}

	if hasStats {
		summary.RequestCount = stats.TotalRequests
		if stats.TotalRequests > 0 {
			successRate := (float64(stats.SuccessfulRequests) * 100.0) / float64(stats.TotalRequests)
			summary.SuccessRate = format.Percentage(successRate)
			// Min/max/avg are only meaningful with traffic; min/max follow the
			// plain-zero convention of RequestCount, avg is a pointer so it is
			// omitted entirely when there is no traffic.
			summary.MinLatencyMs = stats.MinLatency
			summary.MaxLatencyMs = stats.MaxLatency
			avg := stats.AverageLatency
			summary.AvgLatencyMs = &avg
		} else {
			summary.SuccessRate = noTrafficSuccessRate
		}
	} else {
		summary.SuccessRate = noTrafficSuccessRate
	}

	summary.Issues = a.getEndpointIssuesSummaryOptimised(endpoint, stats, hasStats)

	return summary
}

func (a *Application) getEndpointIssuesSummaryOptimised(endpoint *domain.Endpoint, stats ports.EndpointStats, hasStats bool) string {
	if endpoint.Status == domain.StatusHealthy && endpoint.ConsecutiveFailures == 0 {
		return ""
	}
