package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/version"
)

func (a *Application) metricsHandler(w http.ResponseWriter, r *http.Request) {
	snapshot, err := a.gatherStatusSnapshot(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get endpoint data: %v", err), http.StatusInternalServerError)
		return
	}

	response := a.buildStatusResponse(snapshot)

	modelStats := make(map[string]ports.ModelStats)
	modelEndpointStats := make(map[string]map[string]ports.EndpointModelStats)
	if a.statsCollector != nil {
		modelStats = a.statsCollector.GetModelStats()
		modelEndpointStats = a.statsCollector.GetModelEndpointStats()
	}
	if modelStats == nil {
		modelStats = make(map[string]ports.ModelStats)
	}
	if modelEndpointStats == nil {
		modelEndpointStats = make(map[string]map[string]ports.EndpointModelStats)
	}
	models, acc := a.buildModelStats(modelStats, modelEndpointStats, false)
	summary := a.buildSummary(models, modelStats, acc)

	w.Header().Set(constants.HeaderContentType, constants.ContentTypePrometheus)
	w.WriteHeader(http.StatusOK)
	writePrometheusMetrics(w, response, snapshot, a.Config.Proxy, time.Since(a.StartTime), summary, modelStats, modelEndpointStats)
}

func writePrometheusMetrics(w http.ResponseWriter, response StatusResponse, snapshot *statusSnapshot, proxyConfig config.ProxyConfig, uptime time.Duration,
	summary ModelStatsSummary, modelStats map[string]ports.ModelStats, modelEndpointStats map[string]map[string]ports.EndpointModelStats) {
	var b strings.Builder

	writePrometheusHelpType(&b, "olla_info", "gauge", "Olla build and proxy configuration")
	writePrometheusLabeledGauge(&b, "olla_info", 1,
		"version", version.Version,
		"commit", version.Commit,
		"engine", proxyConfig.Engine,
		"profile", proxyConfig.Profile,
		"balancer", proxyConfig.LoadBalancer,
	)

	writePrometheusHelpType(&b, "olla_system_status", "gauge", "Overall system status (2=healthy, 1=degraded, 0=critical)")
	writePrometheusGauge(&b, "olla_system_status", systemStatusValue(response.System.Status))

	writePrometheusHelpType(&b, "olla_endpoints_total", "gauge", "Total configured endpoints")
	writePrometheusGauge(&b, "olla_endpoints_total", float64(len(snapshot.all)))

	writePrometheusHelpType(&b, "olla_endpoints_healthy", "gauge", "Number of healthy endpoints")
	writePrometheusGauge(&b, "olla_endpoints_healthy", float64(len(snapshot.healthy)))

	writePrometheusHelpType(&b, "olla_success_rate_percent", "gauge", "Proxy success rate percentage")
	writePrometheusGauge(&b, "olla_success_rate_percent", parsePercentage(response.System.SuccessRate))

	writePrometheusHelpType(&b, "olla_avg_latency_ms", "gauge", "Average proxy latency in milliseconds")
	writePrometheusGauge(&b, "olla_avg_latency_ms", float64(snapshot.proxyStats.AverageLatency))

	writePrometheusHelpType(&b, "olla_total_traffic_bytes", "gauge", "Total traffic across all endpoints in bytes")
	writePrometheusGauge(&b, "olla_total_traffic_bytes", float64(totalTrafficBytes(snapshot)))

	writePrometheusHelpType(&b, "olla_uptime_seconds", "gauge", "Olla process uptime in seconds")
	writePrometheusGauge(&b, "olla_uptime_seconds", uptime.Seconds())

	writePrometheusHelpType(&b, "olla_active_connections", "gauge", "Active connections across all endpoints")
	writePrometheusGauge(&b, "olla_active_connections", float64(response.System.ActiveConnections))

	writePrometheusHelpType(&b, "olla_security_violations_total", "counter", "Total security violations")
	writePrometheusGauge(&b, "olla_security_violations_total", float64(response.System.SecurityViolations))

	writePrometheusHelpType(&b, "olla_requests_total", "counter", "Total proxy requests processed")
	writePrometheusGauge(&b, "olla_requests_total", float64(response.System.TotalRequests))

	writePrometheusHelpType(&b, "olla_failures_total", "counter", "Total failed proxy requests")
	writePrometheusGauge(&b, "olla_failures_total", float64(response.System.TotalFailures))

	writePrometheusHelpType(&b, "olla_security_blocked_ips", "gauge", "Unique IPs blocked by rate limiting")
	writePrometheusGauge(&b, "olla_security_blocked_ips", float64(response.Security.BlockedIPs))

	writePrometheusHelpType(&b, "olla_security_rate_limit_violations_total", "counter", "Rate limit violations")
	writePrometheusGauge(&b, "olla_security_rate_limit_violations_total", float64(response.Security.Violations.RateLimits))

	writePrometheusHelpType(&b, "olla_security_size_limit_violations_total", "counter", "Request size limit violations")
	writePrometheusGauge(&b, "olla_security_size_limit_violations_total", float64(response.Security.Violations.SizeLimits))

	writePrometheusHelpType(&b, "olla_security_status", "gauge", "Security posture (1=normal, 0=elevated)")
	writePrometheusGauge(&b, "olla_security_status", securityStatusValue(response.Security.Status))

	writeEndpointPrometheusMetrics(&b, snapshot.all, snapshot.endpointStats, snapshot.connectionStats, snapshot.endpointModels)
	writeModelPrometheusMetrics(&b, summary, modelStats, modelEndpointStats)

	_, _ = w.Write([]byte(b.String()))
}

type endpointMetricSample struct {
	name        string
	status      string
	up          float64
	requests    float64
	connections float64
	successRate float64
	avgLatency  float64
	traffic     float64
	priority    float64
	modelCount  float64
}

func writeEndpointPrometheusMetrics(b *strings.Builder, all []*domain.Endpoint, statsMap map[string]ports.EndpointStats,
	connectionStats map[string]int64, modelMap map[string]*domain.EndpointModels) {
	samples := make([]endpointMetricSample, 0, len(all))
	for _, endpoint := range all {
		url := endpoint.GetURLString()
		stats, hasStats := statsMap[url]

		sample := endpointMetricSample{
			name:        endpoint.Name,
			status:      endpoint.Status.String(),
			connections: float64(connectionStats[url]),
			priority:    float64(endpoint.Priority),
		}
		if endpoint.Status == domain.StatusHealthy {
			sample.up = 1
		}
		if hasStats {
			sample.requests = float64(stats.TotalRequests)
			sample.avgLatency = float64(stats.AverageLatency)
			sample.traffic = float64(stats.TotalBytes)
			if stats.TotalRequests > 0 {
				sample.successRate = float64(stats.SuccessfulRequests) / float64(stats.TotalRequests) * 100.0
			}
		}
		if endpointModels := modelMap[url]; endpointModels != nil {
			sample.modelCount = float64(len(endpointModels.Models))
		}
		samples = append(samples, sample)
	}

	writePrometheusHelpType(b, "olla_endpoint_up", "gauge", "Whether the endpoint is healthy (1) or not (0)")
	writePrometheusHelpType(b, "olla_endpoint_requests_total", "counter", "Total requests handled by endpoint")
	writePrometheusHelpType(b, "olla_endpoint_connections", "gauge", "Active connections for endpoint")
	writePrometheusHelpType(b, "olla_endpoint_success_rate_percent", "gauge", "Endpoint success rate percentage")
	writePrometheusHelpType(b, "olla_endpoint_avg_latency_ms", "gauge", "Endpoint average latency in milliseconds")
	writePrometheusHelpType(b, "olla_endpoint_traffic_bytes", "counter", "Total traffic for endpoint in bytes")
	writePrometheusHelpType(b, "olla_endpoint_priority", "gauge", "Endpoint routing priority")
	writePrometheusHelpType(b, "olla_endpoint_models_count", "gauge", "Number of models discovered on endpoint")

	for _, sample := range samples {
		writePrometheusLabeledGauge(b, "olla_endpoint_up", sample.up, "endpoint", sample.name, "status", sample.status)
	}
	for _, sample := range samples {
		writePrometheusLabeledGauge(b, "olla_endpoint_requests_total", sample.requests, "endpoint", sample.name)
	}
	for _, sample := range samples {
		writePrometheusLabeledGauge(b, "olla_endpoint_connections", sample.connections, "endpoint", sample.name)
