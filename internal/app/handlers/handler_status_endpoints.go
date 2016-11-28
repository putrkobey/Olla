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
