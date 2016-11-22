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

	if endpoint.Status == domain.StatusOffline || endpoint.Status == domain.StatusUnhealthy {
		return "unavailable"
	}

	if endpoint.ConsecutiveFailures > 3 {
		return "unstable"
	}

	if hasStats && stats.TotalRequests > 10 {
		if stats.SuccessfulRequests*100 < stats.TotalRequests*90 {
			return "low success rate"
		}
	}

	return ""
}

// stableEndpointID derives the base (no-sibling) stable identifier from the
// SANITISED endpoint URL (scheme+host+port+path only). RawQuery and Fragment
// are NOT part of the hash, so credentials embedded in the query string can
// neither influence the public ID nor leak through it, and credential
// rotation leaves the value unchanged. It is the fallback path for callers
// that have no sibling context (e.g. a stale model-map key with no
// repository match).
//
// Status handlers with the full endpoint set MUST call buildEndpointIDs
// instead: when two configured endpoints collapse to the same sanitised form
// (differing only by query string or fragment), buildEndpointIDs assigns each
// a deterministic positional disambiguator so they remain distinct, which a
// pure per-URL hash cannot do.
//
// FNV-1a 32-bit rendered as base36 mirrors the stableId() hash the dashboard
// already uses client-side for DOM ids (web/dashboard/src/lib/dom-id.js), so
// both layers derive identity the same way. A 32-bit hash is not injective -
// two distinct URLs can theoretically collide - but that is an acceptable,
// collision-resistant trade-off at realistic fleet sizes (tens to low
// hundreds of endpoints), not a correctness requirement.
func stableEndpointID(raw string) string {
	return hashIDString(sanitiseDisplayURL(raw))
}

// hashIDString renders the FNV-1a 32-bit hash of s as base36. The base and
// disambiguated ID paths share this so every ID format is the same shape.
func hashIDString(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return strconv.FormatUint(uint64(h.Sum32()), 36)
}

// buildEndpointIDs derives a stable, opaque ID for every endpoint in the set
// and returns a URL-keyed map for O(1) lookup by the status handlers.
//
// IDs are derived from the SANITISED URL (scheme+host+port+path) so
// query-embedded secrets neither influence the value nor leak through a
// public identifier, and credential rotation leaves the ID unchanged.
//
// Endpoints that collapse to the same sanitised form (differing only by query
// string or fragment) get a deterministic positional suffix "base-N", where N
// is the sibling's position in its collision group. Siblings are sorted on
// the operator-chosen, secret-independent Name field so a credential
// rotation in one sibling's query cannot reshuffle suffixes; the raw URL is
// only a final tiebreaker for the pathological same-Name same-sanitised-URL
// case, where the IDs remain distinct regardless of which sibling gets which
// suffix.
//
// The same input set always yields the same mapping, so /internal/status,
// /internal/status/endpoints and /internal/status/models emit identical IDs
// for the same endpoint as long as they build from the same repository
// snapshot. Nil endpoints are skipped (defensive; the repository does not
// yield nil entries today).
func buildEndpointIDs(endpoints []*domain.Endpoint) map[string]string {
	if len(endpoints) == 0 {
		return map[string]string{}
	}

	// Group endpoints by their sanitised form. Every group larger than one is
	// a collision the display URL cannot break on its own.
	groups := make(map[string][]*domain.Endpoint, len(endpoints))
	for _, ep := range endpoints {
		if ep == nil {
			continue
		}
		key := sanitiseDisplayURL(ep.URLString)
		groups[key] = append(groups[key], ep)
	}

	ids := make(map[string]string, len(endpoints))
	for _, siblings := range groups {
		base := hashIDString(sanitiseDisplayURL(siblings[0].URLString))
		if len(siblings) == 1 {
			ids[siblings[0].URLString] = base
			continue
		}
		// Sort on Name so suffix assignment is secret-independent; raw URL is
		// only a final tiebreaker for the same-Name same-sanitised-URL case,
		// where IDs stay distinct regardless of the suffix each gets.
		// Accepted limitation: when siblings ALSO share the same Name, the
		// raw-URL tiebreak means rotating a credential in one sibling's query
