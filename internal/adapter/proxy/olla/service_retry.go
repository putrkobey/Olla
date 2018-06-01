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
	// if UpdateConfig races concurrently.
	cfg := s.configuration.Load()

	// Check circuit breaker first. attempt is the probe correlation token when
	// this call was admitted as a half-open probe (0 when the circuit was
	// closed); it is threaded through to the recorder calls below so a late
	// result from a superseded probe can't clobber the replacement's state.
	cb := s.GetCircuitBreaker(endpoint.Name)
	var attempt int64
	if cb != nil {
		var open bool
		open, attempt = cb.IsOpen(HalfOpenStaleness)
		if open {
			rlog.Warn("Circuit breaker is open for endpoint", "endpoint", endpoint.Name)
			// Wrap the typed sentinel so ExecuteWithRetry can recognise this as
			// failover-eligible via errors.Is without falling into the generic
			// connection-error path (which would demote persisted health).
			err := fmt.Errorf("%w: endpoint %s", core.ErrCircuitBreakerOpen, endpoint.Name)
			s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), err)
			return err
		}
	}

	// Build target URL using common function that respects preserve_path
	targetURL := common.BuildTargetURL(r, endpoint, cfg.GetProxyPrefix())
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

	// Get endpoint-specific connection pool and transport
	pool := s.getOrCreateEndpointPool(endpoint.Name)
	transport := pool.transport

	// Rewrite model name in request body if this is an alias-resolved request
	core.RewriteModelForAlias(ctx, r, endpoint)

	// log at DEBUG when a model alias rewrite occurred so operators can correlate
	// the alias name the client sent with the actual model dispatched to the backend
	if resolvedModel != stats.Model {
		rlog.Debug("Model alias rewritten for backend",
			"alias", stats.Model,
			"actual_model", resolvedModel,
			"endpoint", endpoint.Name)
	}

	proxyReq, err := s.prepareProxyRequest(ctx, r, targetURL, endpoint, stats)
	if err != nil {
		if cb != nil {
			cb.RecordFailure(attempt)
		}
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), err)
		return fmt.Errorf("failed to create proxy request: %w", err)
	}

	rlog.Debug("making round-trip request", "target", targetURL.String())
	backendStart := time.Now()
	resp, err := transport.RoundTrip(proxyReq)
	stats.BackendResponseMs = time.Since(backendStart).Milliseconds()

	if err != nil {
		if cb != nil {
			cb.RecordFailure(attempt)
		}
		// Don't log as error if it's a connection failure - the retry handler will handle it
		if core.IsConnectionError(err) {
			rlog.Debug("round-trip connection failed", "error", err)
		} else {
			rlog.Error("round-trip failed", "error", err)
		}
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), err)
		duration := time.Since(stats.StartTime)
		return common.MakeUserFriendlyError(err, duration, "backend", cfg.GetResponseTimeout())
	}
	defer resp.Body.Close()

	// Record success with circuit breaker
	if cb != nil {
		cb.RecordSuccess(attempt)
	}

	rlog.Debug("round-trip success", "status", resp.StatusCode)

	core.SetResponseHeaders(w, stats, endpoint)
	core.SetStickySessionHeaders(w, r)

	// Copy response headers, stripping any sensitive headers the upstream may reflect
	core.CopyResponseHeaders(w.Header(), resp.Header, endpoint)

	w.WriteHeader(resp.StatusCode)

	// Stream the response through with Olla's optimizations
	rlog.Debug("starting response stream")
	streamStart := time.Now()
	stats.FirstDataMs = time.Since(stats.StartTime).Milliseconds()

	buffer, poolErr := s.bufferPool.Get()
	if poolErr != nil {
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), poolErr)
		return fmt.Errorf("olla: stream buffer unavailable: %w", poolErr)
	}
	defer s.bufferPool.Put(buffer)

	// Separate client and upstream contexts for proper cancellation handling
	upstreamCtx := ctx
	if resp != nil && resp.Request != nil {
		upstreamCtx = resp.Request.Context()
	}

	// Stream with Olla's optimized streaming
	// Use r.Context() for client context and upstreamCtx for upstream context
	bytesWritten, lastChunk, streamErr := s.streamResponse(r.Context(), upstreamCtx, w, resp, *buffer, rlog)
	stats.StreamingMs = time.Since(streamStart).Milliseconds()
	stats.TotalBytes = bytesWritten

	if streamErr != nil && !errors.Is(streamErr, context.Canceled) {
		rlog.Error("streaming failed", "error", streamErr)
		s.RecordFailure(ctx, endpoint, resolvedModel, time.Since(stats.StartTime), streamErr)
