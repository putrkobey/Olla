package sherpa

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// ProxyRequestToEndpointsWithRetry handles request proxying with automatic failover
func (s *Service) ProxyRequestToEndpointsWithRetry(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	s.IncrementRequests()

	// Use context logger if available, fallback to provided logger
	ctxLogger := middleware.GetLogger(ctx)
	if ctxLogger != nil {
		ctxLogger.Debug("Sherpa proxy request started",
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

	// Delegate to retry handler with endpoint-specific proxy function
	proxyFunc := func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoint *domain.Endpoint, stats *ports.RequestStats) error {
		return s.proxyToSingleEndpoint(ctx, w, r, endpoint, stats, rlog)
	}

	return s.retryHandler.ExecuteWithRetry(ctx, w, r, endpoints, s.Selector, stats, proxyFunc)
}

// proxyToSingleEndpoint executes the proxy request to a specific endpoint
// Note: Connection increment/decrement is handled by RetryHandler.executeProxyAttempt
// to avoid double-counting (see proxy_olla_connection_counting_test.go for context).
func (s *Service) proxyToSingleEndpoint(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoint *domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	stats.EndpointName = endpoint.Name

	targetURL := common.BuildTargetURL(r, endpoint, s.configuration.GetProxyPrefix())

	stats.TargetUrl = targetURL.String()

	// Log request dispatch after target URL is computed
	ctxLogger := middleware.GetLogger(ctx)
	if ctxLogger != nil {
		ctxLogger.Info("Request dispatching",
			"endpoint", endpoint.Name,
			"target", stats.TargetUrl,
			"model", stats.Model)
	} else {
		rlog.Info("Request dispatching", "endpoint", endpoint.Name, "target", stats.TargetUrl, "model", stats.Model)
	}

	// Rewrite model name in request body if this is an alias-resolved request
	core.RewriteModelForAlias(ctx, r, endpoint)

	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, targetURL.String(), r.Body)
	if err != nil {
		s.RecordFailure(ctx, endpoint, core.ResolvedModelName(ctx, endpoint, stats.Model), time.Since(stats.StartTime), err)
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	rlog.Debug("created proxy request")

	headerStart := time.Now()
	core.CopyHeaders(proxyReq, r, endpoint)
	stats.HeaderProcessingMs = time.Since(headerStart).Milliseconds()

	// Add model header if available. The translation layer sets
	// ContextModelKey to the model it resolved (handler_translation.go),
	// which supersedes both the raw request and the alias map for the rest
	// of this function - recompute the resolved-for-stats name below from
	// this updated stats.Model, not the value captured before this point.
	if model, ok := ctx.Value(constants.ContextModelKey).(string); ok && model != "" {
		proxyReq.Header.Set(constants.HeaderXModel, model)
		stats.Model = model
	}

	// The model actually dispatched to this endpoint: the alias map's
	// per-endpoint entry if this was an alias-resolved request, otherwise
	// stats.Model unchanged (which may itself already be the translation
	// layer's resolved name from the ContextModelKey check above). Used for
	// every RecordSuccess/RecordFailure below so /internal/stats/models
	// aggregates under the backend model name, not the client-facing alias.
	resolvedModel := core.ResolvedModelName(ctx, endpoint, stats.Model)

	// We mark the request processing as complete here
	stats.RequestProcessingMs = time.Since(stats.StartTime).Milliseconds()

	rlog.Debug("making round-trip request", "target", targetURL.String())
	backendStart := time.Now()
	resp, err := s.transport.RoundTrip(proxyReq)
	stats.BackendResponseMs = time.Since(backendStart).Milliseconds()

	if err != nil {
		// Suppress error logging for connection failures handled by retry logic
		if core.IsConnectionError(err) {
			rlog.Debug("round-trip connection failed", "error", err)
		} else {
			rlog.Error("round-trip failed", "error", err)
		}
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), err)
		duration := time.Since(stats.StartTime)
		return common.MakeUserFriendlyError(err, duration, "backend", s.configuration.GetResponseTimeout())
	}
	defer resp.Body.Close()

	rlog.Debug("round-trip success", "status", resp.StatusCode)

	core.SetResponseHeaders(w, stats, endpoint)
	core.SetStickySessionHeaders(w, r)

	// Copy response headers, stripping any sensitive headers the upstream may reflect
	core.CopyResponseHeaders(w.Header(), resp.Header, endpoint)

	w.WriteHeader(resp.StatusCode)

	// Stream the response through
	rlog.Debug("starting response stream")
	streamStart := time.Now()
	stats.FirstDataMs = time.Since(stats.StartTime).Milliseconds()

	buffer, poolErr := s.bufferPool.Get()
	if poolErr != nil {
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), poolErr)
