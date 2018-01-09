package ports

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type StatsCollector interface {
	RecordRequest(endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64)
	RecordConnection(endpoint *domain.Endpoint, delta int) // +1 connect, -1 disconnect
	RecordSecurityViolation(violation SecurityViolation)
	RecordDiscovery(endpoint *domain.Endpoint, success bool, latency time.Duration)

	// Model-specific tracking
	RecordModelRequest(model string, endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64)
	RecordModelError(model string, endpoint *domain.Endpoint, errorType string)
	GetModelStats() map[string]ModelStats
	GetModelEndpointStats() map[string]map[string]EndpointModelStats

	// Translator-specific tracking
	RecordTranslatorRequest(event TranslatorRequestEvent)
	GetTranslatorStats() map[string]TranslatorStats

	GetProxyStats() ProxyStats
	GetEndpointStats() map[string]EndpointStats
	GetSecurityStats() SecurityStats
	GetConnectionStats() map[string]int64
	// GetConnectionCount returns the active connection count for a single endpoint.
	// Prefer this over GetConnectionStats() when only one endpoint's count is
	// needed - it avoids building a map of every tracked endpoint just to read
	// one entry, which matters on selector hot paths called once per candidate.
	GetConnectionCount(endpoint string) int64
}

type EndpointStats struct {
	Name               string  `json:"name"`
	URL                string  `json:"url"`
	ActiveConnections  int64   `json:"active_connections"`
	TotalRequests      int64   `json:"total_requests"`
