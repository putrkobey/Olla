package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"hash"
	"hash/fnv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/version"

	"github.com/putrkobey/Olla/internal/config"

	"github.com/putrkobey/Olla/internal/util"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/pkg/format"
)

var (
	statusHealthy  = "healthy"
	statusDegraded = "degraded"
	statusCritical = "critical"
	statusNormal   = "normal"
	statusElevated = "elevated"
	zeroTraffic    = "0 B"
	emptyString    = ""
)

type SystemSummary struct {
	// Additive field, keep JSON contract backward-compatible: absolute process
	// start so the dashboard can compute a live uptime between polls without
	// refetching. Not omitempty: always known once the process is up. Sits
	// alongside the existing relative uptime string.
	StartTime          time.Time `json:"start_time"`
	Status             string    `json:"status"`
	EndpointsUp        string    `json:"endpoints_up"`
	SuccessRate        string    `json:"success_rate"`
	AvgLatency         string    `json:"avg_latency"`
	TotalTraffic       string    `json:"total_traffic"`
	UptimeHuman        string    `json:"uptime"`
	Version            string    `json:"version"`
	Commit             string    `json:"commit"`
	ActiveConnections  int64     `json:"active_connections"`
	SecurityViolations int64     `json:"security_violations"`
	TotalRequests      int64     `json:"total_requests"`
	TotalFailures      int64     `json:"total_failures"`
	// HasTraffic lets the dashboard branch on the no-traffic state without
	// parsing SuccessRate. Always present (not omitempty): it is derivable from
	// TotalRequests, which is itself always emitted, so every payload carries
	// an explicit true/false. SuccessRate becomes "N/A" when this is false,
	// mirroring the endpoint-level convention in handler_status_endpoints.go;
	// previously it reported "0%" on a healthy fresh boot, which coupled with
	// the legacy status threshold produced a misleading "critical" verdict.
	HasTraffic bool `json:"has_traffic"`
}

type ProxySummary struct {
	Engine   string `json:"engine"`
	Profile  string `json:"profile"`
	Balancer string `json:"balancer"`
}

type EndpointResponse struct {
	AvgLatencyMs  *int64                 `json:"avg_latency_ms,omitempty"`
	NextCheckAt   *time.Time             `json:"next_check_at,omitempty"`
	HealthCheckAt *time.Time             `json:"health_check_at,omitempty"`
	Models        EndpointModelsResponse `json:"models"`
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	SuccessRate   string                 `json:"success_rate"`
	AvgLatency    string                 `json:"avg_latency"`
	Traffic       string                 `json:"traffic"`
	LastCheck     string                 `json:"last_check"`
	NextCheck     string                 `json:"next_check"`
	Issues        string                 `json:"issues"`
	URL           string                 `json:"url"`
	// ID is a stable, opaque identifier derived from the SANITISED endpoint
	// URL via buildEndpointIDs; see handler_status_endpoints.go.
	ID          string `json:"id"`
	Priority    int    `json:"priority"`
	Connections int64  `json:"connections"`
	Requests    int64  `json:"requests"`
	// Additive dashboard fields, existing fields above are unchanged.
	// active_connections is intentionally NOT duplicated here: Connections
	// already carries the same value. last_model_sync_at is intentionally NOT
	// duplicated: Models.LastUpdated already serialises as an RFC3339 absolute.
	MinLatencyMs int64 `json:"min_latency_ms"`
	MaxLatencyMs int64 `json:"max_latency_ms"`
}

type EndpointModelsResponse struct {
	LastUpdated time.Time `json:"last_updated"`
	Count       int64     `json:"count"`
}

type SecuritySummary struct {
	Status     string            `json:"status"`
	BlockedIPs int               `json:"blocked_ips"`
	Violations SecurityViolation `json:"violations"`
}
type SecurityViolation struct {
	RateLimits int64 `json:"rate_limits"`
	SizeLimits int64 `json:"size_limits"`
}
type StatusResponse struct {
	Timestamp time.Time          `json:"timestamp"`
	Proxy     ProxySummary       `json:"proxy"`
	Endpoints []EndpointResponse `json:"endpoints"`
	Security  SecuritySummary    `json:"security"`
	System    SystemSummary      `json:"system"`
}

type statusSnapshot struct {
	endpointStats   map[string]ports.EndpointStats
	connectionStats map[string]int64
	endpointModels  map[string]*domain.EndpointModels
	all             []*domain.Endpoint
	healthy         []*domain.Endpoint
	proxyStats      ports.ProxyStats
	securityStats   ports.SecurityStats
}

func (a *Application) gatherStatusSnapshot(ctx context.Context) (*statusSnapshot, error) {
	all, healthy, _, err := a.getEndpointCounts(ctx)
	if err != nil {
		return nil, err
	}

	endpointModelsMap, emErr := a.modelRegistry.GetEndpointModelMap(ctx)
	if emErr != nil {
		a.logger.Warn("Failed to get model map", "error", emErr)
		endpointModelsMap = make(map[string]*domain.EndpointModels)
	}

	var endpointStats map[string]ports.EndpointStats
	var proxyStats ports.ProxyStats
	var securityStats ports.SecurityStats
	var connectionStats map[string]int64

	if a.statsCollector != nil {
		endpointStats = a.statsCollector.GetEndpointStats()
		proxyStats = a.statsCollector.GetProxyStats()
		securityStats = a.statsCollector.GetSecurityStats()
		connectionStats = a.statsCollector.GetConnectionStats()
	}

	return &statusSnapshot{
		all:             all,
		healthy:         healthy,
		endpointStats:   endpointStats,
		proxyStats:      proxyStats,
		securityStats:   securityStats,
		connectionStats: connectionStats,
		endpointModels:  endpointModelsMap,
	}, nil
}

func (a *Application) buildStatusResponse(snapshot *statusSnapshot) StatusResponse {
	response := StatusResponse{
		Timestamp: time.Now(),
		Endpoints: make([]EndpointResponse, len(snapshot.all)),
	}

	response.Proxy = a.buildProxySummary(a.Config.Proxy)
	response.System = a.buildSystemSummary(snapshot.all, snapshot.healthy, snapshot.proxyStats, snapshot.securityStats, snapshot.connectionStats, snapshot.endpointStats)
	// IDs derive once from the same snapshot so every payload agrees and
	// colliding siblings stay distinct via positional disambiguators.
	endpointIDs := buildEndpointIDs(snapshot.all)
	a.buildUnifiedEndpoints(snapshot.all, snapshot.endpointStats, snapshot.connectionStats, response.Endpoints, snapshot.endpointModels, endpointIDs)
	response.Security = a.buildSecuritySummary(snapshot.securityStats)

	return response
}

func (a *Application) statusHandler(w http.ResponseWriter, r *http.Request) {
	snapshot, err := a.gatherStatusSnapshot(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get endpoint data: %v", err), http.StatusInternalServerError)
		return
	}

	response := a.buildStatusResponse(snapshot)

	body, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
	writeJSONWithETag(w, r, body, hashStatusResponse(&response))
}

func (a *Application) buildProxySummary(proxyConfig config.ProxyConfig) ProxySummary {
	return ProxySummary{
		Engine:   proxyConfig.Engine,
		Profile:  proxyConfig.Profile,
		Balancer: proxyConfig.LoadBalancer,
	}
}

func (a *Application) buildSystemSummary(all, healthy []*domain.Endpoint, proxy ports.ProxyStats, security ports.SecurityStats, connections map[string]int64, endpointStats map[string]ports.EndpointStats) SystemSummary {
	var totalConnections, totalTraffic int64

	for url, conn := range connections {
		totalConnections += conn
		if stats, exists := endpointStats[url]; exists {
			totalTraffic += stats.TotalBytes
		}
	}

	hasTraffic := proxy.TotalRequests > 0

	var status string
	if len(all) == 0 {
		// len(healthy)/len(all) is 0/0 = NaN with zero endpoints, and every NaN
		// comparison below is false, so both branches below would fall through
		// to their default healthy case. A proxy with nothing to route to
		// cannot be healthy regardless of traffic, so this is decided before
		// the ratio is even computed.
		status = statusCritical
	} else {
		healthyRatio := float64(len(healthy)) / float64(len(all))
		if hasTraffic {
			// Dual-threshold verdict: a healthy fleet cannot mask a failing proxy
			// and high failure rates cannot mask healthy endpoints.
			systemSuccessRate := float64(proxy.SuccessfulRequests) / float64(proxy.TotalRequests) * 100.0
			switch {
			case healthyRatio < 0.5 || systemSuccessRate < 90.0:
				status = statusCritical
			case healthyRatio < 0.8 || systemSuccessRate < 95.0:
				status = statusDegraded
			default:
				status = statusHealthy
			}
		} else {
			// Fresh boot (or no requests yet): the proxy success rate is undefined,
			// so derive status purely from endpoint health. Without this branch a
			// perfectly healthy zero-traffic fleet fell through to the legacy
			// < 90.0 success-rate clause and reported critical.
			switch {
			case healthyRatio < 0.5:
				status = statusCritical
			case healthyRatio < 0.8:
				status = statusDegraded
			default:
				status = statusHealthy
			}
		}
	}

	totalViolations := security.RateLimitViolations + security.SizeLimitViolations

	// No traffic means the success rate is undefined; noTrafficSuccessRate
	// mirrors the endpoint-level convention in handler_status_endpoints.go and
	// stops the dashboard rendering a misleading "0%" on a fresh boot.
	successRateStr := noTrafficSuccessRate
	if hasTraffic {
		systemSuccessRate := float64(proxy.SuccessfulRequests) / float64(proxy.TotalRequests) * 100.0
		successRateStr = format.Percentage(systemSuccessRate)
	}

	return SystemSummary{
		Version:            version.Version,
		Commit:             version.Commit,
		Status:             status,
		EndpointsUp:        format.EndpointsUp(len(healthy), len(all)),
		SuccessRate:        successRateStr,
		AvgLatency:         format.Latency(proxy.AverageLatency),
		ActiveConnections:  totalConnections,
		SecurityViolations: totalViolations,
		TotalTraffic:       format.Bytes(util.SafeUint64(totalTraffic)),
		TotalRequests:      proxy.TotalRequests,
		TotalFailures:      proxy.FailedRequests,
		UptimeHuman:        format.Duration2(time.Since(a.StartTime)),
		StartTime:          a.StartTime,
		HasTraffic:         hasTraffic,
	}
}

func (a *Application) buildUnifiedEndpoints(all []*domain.Endpoint, statsMap map[string]ports.EndpointStats,
	connectionStats map[string]int64, endpoints []EndpointResponse, modelMap map[string]*domain.EndpointModels, endpointIDs map[string]string) {
	for i, endpoint := range all {
		url := endpoint.GetURLString()
		stats, hasStats := statsMap[url]
		connections := connectionStats[url]
		endpointModels := modelMap[url]

		var successRate float64
		if hasStats && stats.TotalRequests > 0 {
			successRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests) * 100.0
		}

		traffic := zeroTraffic
		requests := int64(0)
		avgLatency := int64(0)
		if hasStats {
			traffic = format.Bytes(util.SafeUint64(stats.TotalBytes))
			requests = stats.TotalRequests
			avgLatency = stats.AverageLatency
		}

		var modelDisco EndpointModelsResponse
		if endpointModels != nil {
			modelDisco = EndpointModelsResponse{
				LastUpdated: endpointModels.LastUpdated,
				Count:       int64(len(endpointModels.Models)),
			}
		}

		endpoints[i] = EndpointResponse{
			Name:        endpoint.Name,
			Status:      endpoint.Status.String(),
			Priority:    endpoint.Priority,
			Connections: connections,
			Requests:    requests,
			SuccessRate: format.Percentage(successRate),
