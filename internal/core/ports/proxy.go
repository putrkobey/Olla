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
