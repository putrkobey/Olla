package ports

import (
	"context"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// ProxyService defines the interface for the proxy service
type ProxyService interface {
	ProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, stats *RequestStats, rlog logger.StyledLogger) error
	ProxyRequestToEndpoints(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *RequestStats, rlog logger.StyledLogger) error
	GetStats(ctx context.Context) (ProxyStats, error)
	UpdateConfig(configuration ProxyConfiguration)
}

// ProxyStats contains statistics about the proxy service
type ProxyStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	AverageLatency     int64 // in milliseconds
	MinLatency         int64
	MaxLatency         int64
}

// ProxyConfiguration allows us to reuse the configuration across multiple proxy implementations
type ProxyConfiguration interface {
	GetProxyPrefix() string
	GetConnectionTimeout() time.Duration
	GetConnectionKeepAlive() time.Duration
	GetResponseTimeout() time.Duration
	GetReadTimeout() time.Duration
	GetStreamBufferSize() int
	GetProxyProfile() string
}

type ProxyFactory interface {
	Create(proxyTpe string, discoveryService DiscoveryService, selector domain.EndpointSelector, config ProxyConfiguration) (ProxyService, error)
	GetAvailableTypes() []string
}

type RequestStats struct {
	StartTime time.Time
	EndTime   time.Time

	// Provider metrics extracted from response (best effort)
	ProviderMetrics *domain.ProviderMetrics      `json:"provider_metrics,omitempty"`
