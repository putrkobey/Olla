package olla

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// ProxyRequestToEndpointsWithRetry proxies the request with retry logic for connection failures
func (s *Service) ProxyRequestToEndpointsWithRetry(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	s.IncrementRequests()

	// Use context logger if available
	ctxLogger := middleware.GetLogger(ctx)
	if ctxLogger != nil {
		ctxLogger.Debug("Olla proxy request started",
			"method", r.Method,
			"url", r.URL.String(),
			"endpoint_count", len(endpoints))
	} else {
		rlog.Debug("proxy request started", "method", r.Method, "url", r.URL.String())
	}

	if len(endpoints) == 0 {
		if ctxLogger != nil {
			ctxLogger.Error("No healthy endpoints available for request")
		} else {
			rlog.Error("no healthy endpoints available")
		}
		s.RecordFailure(ctx, nil, stats.Model, time.Since(stats.StartTime), common.ErrNoHealthyEndpoints)
		return common.ErrNoHealthyEndpoints
	}

	// Define the proxy function for a single endpoint
	proxyFunc := func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoint *domain.Endpoint, stats *ports.RequestStats) error {
		return s.proxyToSingleEndpoint(ctx, w, r, endpoint, stats, rlog)
	}

	// Use the shared retry handler
	return s.retryHandler.ExecuteWithRetry(ctx, w, r, endpoints, s.Selector, stats, proxyFunc)
}

// proxyToSingleEndpoint handles proxying to a single endpoint with Olla's optimizations
func (s *Service) proxyToSingleEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoint *domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	stats.EndpointName = endpoint.Name

	// The model actually dispatched to this endpoint, resolved from the alias
	// map up front so every RecordSuccess/RecordFailure call below (including
	// the failure paths ahead of the alias body rewrite) records model-level
	// stats under the backend model name, not whatever alias the client sent.
	resolvedModel := core.ResolvedModelName(ctx, endpoint, stats.Model)

	// Snapshot config once for this request so all reads below are coherent even
