package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/pkg/format"
)

const (
	queryValueTrue = "true"
)

// safeInt64ToUint64 safely converts int64 to uint64 for byte formatting
func safeInt64ToUint64(n int64) uint64 {
	if n < 0 {
		return 0
	}
	return uint64(n)
}

type ModelStatsResponse struct {
	Timestamp time.Time                     `json:"timestamp"`
	Endpoints map[string]EndpointModelStats `json:"endpoints"`
	Models    []ModelStats                  `json:"models"`
	Summary   ModelStatsSummary             `json:"summary"`
}

type ModelStats struct {
	EndpointBreakdown  map[string]EndpointModelStats `json:"endpoint_breakdown,omitempty"`
	Name               string                        `json:"name"`
	SuccessRate        string                        `json:"success_rate"`
	TotalTraffic       string                        `json:"total_traffic"`
	AverageLatency     string                        `json:"average_latency"`
	P95Latency         string                        `json:"p95_latency"`
	P99Latency         string                        `json:"p99_latency"`
	LastRequested      string                        `json:"last_requested"`
	TotalRequests      int64                         `json:"total_requests"`
	SuccessfulRequests int64                         `json:"successful_requests"`
	FailedRequests     int64                         `json:"failed_requests"`
	UniqueClients      int64                         `json:"unique_clients"`
	RoutingHits        int64                         `json:"routing_hits"`
	RoutingMisses      int64                         `json:"routing_misses"`
	RoutingFallbacks   int64                         `json:"routing_fallbacks"`
}

type EndpointModelStats struct {
	EndpointName      string  `json:"endpoint_name"`
	AverageLatency    string  `json:"average_latency"`
	LastUsed          string  `json:"last_used"`
	RequestCount      int64   `json:"request_count"`
	SuccessRate       float64 `json:"success_rate"`
	ConsecutiveErrors int     `json:"consecutive_errors"`
}

type ModelStatsSummary struct {
	OverallSuccessRate string `json:"overall_success_rate"`
	TotalTraffic       string `json:"total_traffic"`
	MostPopularModel   string `json:"most_popular_model"`
	LeastPopularModel  string `json:"least_popular_model"`
	TotalModels        int    `json:"total_models"`
	ActiveModels       int    `json:"active_models"`
	TotalRequests      int64  `json:"total_requests"`
}

// modelStatsAccumulator tracks accumulated statistics
type modelStatsAccumulator struct {
	mostPopular        string
	leastPopular       string
	totalRequests      int64
	successfulRequests int64
	totalBytes         int64
	maxRequests        int64
	minRequests        int64
}

func (a *Application) modelStatsHandler(w http.ResponseWriter, r *http.Request) {
	statsCollector := a.statsCollector
	if statsCollector == nil {
		http.Error(w, "Stats collector not initialized", http.StatusServiceUnavailable)
		return
	}

	modelStats := statsCollector.GetModelStats()
	modelEndpointStats := statsCollector.GetModelEndpointStats()

	includeEndpoints := r.URL.Query().Get("include_endpoints") == queryValueTrue
	includeSummary := r.URL.Query().Get("include_summary") == queryValueTrue

	models, accumulator := a.buildModelStats(modelStats, modelEndpointStats, includeEndpoints)
	summary := a.buildSummary(models, modelStats, accumulator)
	endpoints := a.buildEndpointSummary(modelEndpointStats, includeSummary)

	response := ModelStatsResponse{
		Timestamp: time.Now(),
		Models:    models,
		Endpoints: endpoints,
		Summary:   summary,
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (a *Application) buildModelStats(modelStats map[string]ports.ModelStats,
	modelEndpointStats map[string]map[string]ports.EndpointModelStats,
	includeEndpoints bool) ([]ModelStats, *modelStatsAccumulator) {

	models := make([]ModelStats, 0, len(modelStats))
	acc := &modelStatsAccumulator{
		minRequests: int64(^uint64(0) >> 1),
	}

	for name, stats := range modelStats {
		model := a.convertModelStats(name, stats, modelEndpointStats, includeEndpoints)
		models = append(models, model)

		// Update accumulator
		acc.totalRequests += stats.TotalRequests
		acc.successfulRequests += stats.SuccessfulRequests
		acc.totalBytes += stats.TotalBytes

		if stats.TotalRequests > acc.maxRequests {
			acc.maxRequests = stats.TotalRequests
			acc.mostPopular = name
		}
		if stats.TotalRequests < acc.minRequests && stats.TotalRequests > 0 {
			acc.minRequests = stats.TotalRequests
			acc.leastPopular = name
		}
	}

	// Sort models by request count (most popular first)
	sort.Slice(models, func(i, j int) bool {
