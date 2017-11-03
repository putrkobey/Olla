package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/util"
)

type proxyRequest struct {
	requestLogger            logger.StyledLogger
	stats                    *ports.RequestStats
	profile                  *domain.RequestProfile
	endpointPaths            map[string]string
	clientIP                 string
	targetPath               string
	model                    string
	contentType              string
	method                   string
	path                     string
	query                    string
	userAgent                string
	translatorMode           constants.TranslatorMode
	translatorFallbackReason string
	// stickyOutcome, stickySource, and sessionID are populated after endpoint
	// selection so the routing outcome is visible in completed-request log lines.
	// sessionID must only appear at DEBUG because client-supplied IDs are user data.
	stickyOutcome string
	stickySource  string
	sessionID     string
	contentLength int64
	// upstreamStatus is the HTTP status the client received for a request that reached the
	// proxy engine; 0 when unknown. Logged so a backend's own 4xx/5xx is visible per request.
	upstreamStatus int
	hadError       bool
	isStreaming    bool
}

func (a *Application) proxyHandler(w http.ResponseWriter, r *http.Request) {
	pr := a.initializeProxyRequest(r)

	ctx, r := a.setupRequestContext(r, pr.stats)

	a.analyzeRequest(ctx, r, pr)

	// Sticky session key must be computed after analyzeRequest so the model
	// name is available; inject into context before endpoint selection.
	// The outcome pointer is stored in context; the proxy engine reads it before WriteHeader.
	if a.Config.Proxy.StickySessions.Enabled {
		ctx, r, _ = a.injectStickyKey(ctx, r, a.canonicalModelName(pr.model))
	}

	endpoints, err := a.getCompatibleEndpoints(ctx, pr)
	if err != nil {
		a.handleEndpointError(w, pr, err)
		return
	}

	a.dispatchToEndpoints(ctx, w, r, pr, endpoints, "")
}

// dispatchToEndpoints is the shared tail of proxyHandler and providerProxyHandler:
// forward to the proxy engine when endpoints were selected, or fail fast with the
// correct status when selection produced none. Consolidating this here means a
// routing rejection (#191) is honoured identically regardless of which route the
// request came in on, instead of each handler drifting its own empty-endpoint handling.
func (a *Application) dispatchToEndpoints(ctx context.Context, w http.ResponseWriter, r *http.Request, pr *proxyRequest, endpoints []*domain.Endpoint, providerType string) {
	if len(endpoints) == 0 {
		a.writeNoRoutableEndpoints(w, r, pr, providerType)
		return
	}

	// Strip the route prefix before forwarding to the backend.
	// Without this, BuildTargetURL receives the full /olla/proxy/... path and
	// GetProxyPrefix() returns "route_prefix" (a context key name, not a URL path),
	// so its StripPrefix is a no-op.
	r.URL.Path = pr.targetPath

	a.logRequestStart(pr, len(endpoints))

	// Wrap w once so both the proxy engine and handleProxyError observe the same
	// commit signal. RetryHandler.ExecuteWithRetry wraps it again internally, which
	// is harmless: both trackers read the same underlying writes, and Unwrap() lets
	// http.NewResponseController chain through both layers to reach the real flusher.
	tracker := core.NewResponseStartedWriter(w)

	err := a.executeProxyRequest(ctx, tracker, r, endpoints, pr)

	// The error response is written first so the logged status is what the client actually
	// received (a 502 for a connection failure), not 0.
	if err != nil {
		a.handleProxyError(tracker, err)
	}
	pr.upstreamStatus = tracker.Status()
	pr.captureStickyOutcome(ctx, r)
	a.logRequestResult(pr, err)
}

// writeNoRoutableEndpoints short-circuits a request when endpoint selection produced
// zero candidates, instead of letting it fall through to the proxy engine (which would
// either proxy to a compatible-but-wrong backend, or return a generic 502/404 that hides
// the actual routing verdict). A rejection routing decision takes priority: it carries
// the precise status code and reason (strict model_not_found gives 404). Without a
// decision we keep the historical per-route defaults.
func (a *Application) writeNoRoutableEndpoints(w http.ResponseWriter, r *http.Request, pr *proxyRequest, providerType string) {
	var decision *domain.ModelRoutingDecision
	if pr.profile != nil {
		decision = pr.profile.RoutingDecision
	}

	var status int
	var reason string

	switch {
	case decision != nil && decision.StatusCode >= http.StatusBadRequest:
		status = decision.StatusCode
		reason = decision.Reason
		pr.stats.RoutingDecision = decision
	case providerType != "":
		// no decision was recorded (e.g. modelRegistry unset), so preserve the
		// precise provider-route message rather than a vague generic one.
		status = http.StatusNotFound
		reason = fmt.Sprintf("No %s endpoints available", providerType)
	default:
		status = http.StatusServiceUnavailable
		reason = "no healthy endpoints available"
	}

	// Preserve normal request telemetry (client_ip, model, duration, routing fields)
	// even though we're short-circuiting before the proxy engine ever runs. Logged as
	// an explicit rejection rather than logRequestResult's "completed", so a fail-fast
	// 404/503 is never mistaken for a successful proxy in the logs (#191).
	a.logRequestStart(pr, 0)
	pr.captureStickyOutcome(r.Context(), r)
	a.logRequestRejected(pr, status, reason)

	// Headers must be set before http.Error, which calls WriteHeader.
	a.setStickyResponseHeadersFromRequest(w, r)
	a.setRoutingDecisionHeaders(w, decision)

	http.Error(w, reason, status)
}

// setRoutingDecisionHeaders writes the X-Olla-Routing-* observability headers from
// a routing decision. Shared by writeNoRoutableEndpoints and the translation route's
// zero-endpoint rejection so a decision-aware rejection carries the same headers
// regardless of which route produced it (#191).
func (a *Application) setRoutingDecisionHeaders(w http.ResponseWriter, decision *domain.ModelRoutingDecision) {
	if decision == nil {
		return
	}
	w.Header().Set(constants.HeaderXOllaRoutingStrategy, decision.Strategy)
	w.Header().Set(constants.HeaderXOllaRoutingDecision, decision.Action)
	if decision.Reason != "" {
		w.Header().Set(constants.HeaderXOllaRoutingReason, decision.Reason)
	}
}

func (a *Application) initializeProxyRequest(r *http.Request) *proxyRequest {
	// get the requestID from the middleware context first
	requestID := ""
	if id, ok := r.Context().Value(middleware.RequestIDKey).(string); ok {
		requestID = id
	}

	// fallback to generating a new one otherwise
	if requestID == "" {
		requestID = util.GenerateRequestID()
	}

	stats := &ports.RequestStats{
		RequestID: requestID,
		StartTime: time.Now(),
	}

	return &proxyRequest{
		stats:         stats,
		requestLogger: a.logger.WithRequestID(stats.RequestID),
		contentType:   r.Header.Get(constants.HeaderContentType),
		method:        r.Method,
		path:          r.URL.Path,
		query:         r.URL.RawQuery,
		contentLength: r.ContentLength,
		userAgent:     r.UserAgent(),
	}
}

func (a *Application) setupRequestContext(r *http.Request, stats *ports.RequestStats) (context.Context, *http.Request) {
	ctx := context.WithValue(r.Context(), constants.ContextRequestIdKey, stats.RequestID)
	ctx = context.WithValue(ctx, constants.ContextRequestTimeKey, stats.StartTime)
	return ctx, r.WithContext(ctx)
}

// modelCanonicaliser is implemented by registries that know which native name a bare
// Ollama model name resolves to.
type modelCanonicaliser interface {
	CanonicalModelName(name string) string
}

// canonicalModelName names the model a request resolves to, so "foo" and "foo:latest"
// share a stats row and a sticky key instead of splitting one conversation in two. It
// is for keys and labels only; the forwarded body keeps the client's spelling. A
// configured model alias is left alone because its own routing decides the target.
func (a *Application) canonicalModelName(name string) string {
	if name == "" {
		return name
	}
	canon, ok := a.modelRegistry.(modelCanonicaliser)
	if !ok {
		return name
	}
	if a.aliasResolver != nil && a.aliasResolver.IsAlias(name) {
		return name
	}
	return canon.CanonicalModelName(name)
}

// withCanonicalAliasTargets records the native name of any alias target that is a bare
// Ollama name, so stats key on "all-minilm:latest" however the model was requested.
// The alias map itself is untouched: the backend still receives the target as configured.
func (a *Application) withCanonicalAliasTargets(ctx context.Context, requested string, aliasMap map[string]string) context.Context {
	var targets map[string]string
	for _, target := range aliasMap {
		if native := a.canonicalModelName(target); native != target {
			if targets == nil {
				targets = make(map[string]string)
			}
			targets[target] = native
		}
	}
	if targets == nil {
		return ctx
	}
	canonical, _ := ctx.Value(constants.ContextCanonicalModelKey).(core.CanonicalModel)
	canonical.Requested = requested
	canonical.Targets = targets
	return context.WithValue(ctx, constants.ContextCanonicalModelKey, canonical)
}

func (a *Application) analyzeRequest(ctx context.Context, r *http.Request, pr *proxyRequest) {
	pr.requestLogger.Debug("Proxy handler called", "path", r.URL.Path, "method", r.Method)

	rl := a.Config.Server.RateLimits
	pr.clientIP = util.GetClientIP(r, rl.TrustProxyHeaders, rl.TrustedProxyCIDRsParsed)

	pathResolutionStart := time.Now()
	pr.targetPath = a.stripRoutePrefix(ctx, r.URL.Path)

	// inspector chain figures out which endpoints can handle this request (ollama vs openai)
	// and extracts model requirements. failures here are non-fatal - we'll spray and pray
	profile, err := a.inspectorChain.Inspect(ctx, r, pr.targetPath)
	if err != nil {
		pr.requestLogger.Warn("Request inspection failed, continuing with all endpoints", "error", err)
	}
	pr.profile = profile

	if profile != nil && profile.ModelName != "" {
		pr.model = profile.ModelName
		pr.stats.Model = pr.model
	}

	pr.stats.PathResolutionMs = time.Since(pathResolutionStart).Milliseconds()
}

// captureStickyOutcome copies the sticky session result and the client-supplied session id
// into pr so logRequestResult surfaces them. Every handler that logs a completed request
// must call this after endpoint selection; without it, provider routes silently drop the
// fields from their "Request completed" log line (#178, regression from #139).
// The outcome pointer is allocated in injectStickyKeyWithBody and filled by the balancer
// wrapper before Select() returns, so no lock is needed here.
func (pr *proxyRequest) captureStickyOutcome(ctx context.Context, r *http.Request) {
	if outcome, ok := ctx.Value(constants.ContextStickyOutcomeKey).(*balancer.StickyOutcome); ok && outcome != nil {
		pr.stickyOutcome = outcome.Result
		pr.stickySource = outcome.Source
	}
	pr.sessionID = r.Header.Get(constants.HeaderXOllaSessionID)
}

// injectStickyKey computes the affinity key for this request and injects it into the context.
// It reads up to prefix_hash_bytes of the request body (for prefix hashing) then restores the
// body so downstream handlers see it intact. The StickyOutcome pointer lets the wrapper report
// its hit/miss/repin decision back to the handler without an extra context lookup.
func (a *Application) injectStickyKey(ctx context.Context, r *http.Request, modelName string) (context.Context, *http.Request, *balancer.StickyOutcome) {
	cfg := a.Config.Proxy.StickySessions

	// Read a small prefix of the body for prefix_hash; restore it afterwards.
	// We cap at cfg.PrefixHashBytes+1 to handle the case where the body is exactly
	// that length without allocating an oversized buffer.
	var bodySnap []byte
	if r.Body != nil && r.ContentLength != 0 {
		limit := cfg.PrefixHashBytes
		if limit <= 0 {
			limit = 512
		}
		snap, readErr := io.ReadAll(io.LimitReader(r.Body, int64(limit)+1))
		// Restore any bytes already consumed, even on partial-read error, so the
		// downstream proxy always sees a complete body.
		if len(snap) > 0 {
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(snap), r.Body))
		}
		if readErr == nil {
			bodySnap = snap
		}
	}

	return a.injectStickyKeyWithBody(ctx, r, modelName, bodySnap)
}

// injectStickyKeyWithBody is the core sticky key injection path when the body has
// already been buffered by the caller (e.g. the translation handler reads the full
// body to extract the model name before we reach this point). Passing the bytes in
// avoids a second read/restore cycle on the same reader.
func (a *Application) injectStickyKeyWithBody(ctx context.Context, r *http.Request, modelName string, body []byte) (context.Context, *http.Request, *balancer.StickyOutcome) {
	cfg := a.Config.Proxy.StickySessions
	stickyKey, stickySource := balancer.ComputeStickyKey(r, modelName, cfg, body)

	outcome := &balancer.StickyOutcome{}
	ctx = context.WithValue(ctx, constants.ContextStickyKeyKey, stickyKey)
	ctx = context.WithValue(ctx, constants.ContextStickyKeySourceKey, stickySource)
	ctx = context.WithValue(ctx, constants.ContextStickyOutcomeKey, outcome)
	r = r.WithContext(ctx)

	return ctx, r, outcome
}

// setStickyResponseHeadersFromRequest reads the StickyOutcome from the request context
// and writes the sticky session headers. Used by sub-handlers that have *http.Request
// but not the outcome pointer directly. Must be called before w.WriteHeader().
func (a *Application) setStickyResponseHeadersFromRequest(w http.ResponseWriter, r *http.Request) {
	outcome, _ := r.Context().Value(constants.ContextStickyOutcomeKey).(*balancer.StickyOutcome)
	a.setStickyResponseHeaders(w, r, outcome)
}

// setStickyResponseHeaders writes sticky session outcome headers so clients can observe
// affinity routing decisions. When the client provided an explicit session ID header,
// we echo it back so stateless clients can track their own session.
func (a *Application) setStickyResponseHeaders(w http.ResponseWriter, r *http.Request, outcome *balancer.StickyOutcome) {
	if outcome == nil {
		return
	}
	if outcome.Result != "" {
		w.Header().Set(constants.HeaderXOllaStickySession, outcome.Result)
	}
	if outcome.Source != "" && outcome.Source != "none" {
		w.Header().Set(constants.HeaderXOllaStickyKeySource, outcome.Source)
	}
	if outcome.Source == "session_header" {
		if sid := r.Header.Get(constants.HeaderXOllaSessionID); sid != "" {
			w.Header().Set(constants.HeaderXOllaSessionID, sid)
		}
	}
}

func (a *Application) getCompatibleEndpoints(ctx context.Context, pr *proxyRequest) ([]*domain.Endpoint, error) {
	endpoints, err := a.discoveryService.GetHealthyEndpoints(ctx)
	if err != nil {
		pr.requestLogger.Error("Failed to get healthy endpoints", "error", err)
		return nil, fmt.Errorf("no healthy endpoints available: %w", err)
	}

	compatibleEndpoints := a.filterEndpointsByProfile(endpoints, pr.profile, pr.requestLogger)

	return compatibleEndpoints, nil
}

func (a *Application) executeProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, pr *proxyRequest) error {
	ctx, r = a.prepareProxyContext(ctx, r, pr)

	return a.proxyService.ProxyRequestToEndpoints(ctx, w, r, endpoints, pr.stats, pr.requestLogger)
}

func (a *Application) logRequestStart(pr *proxyRequest, endpointCount int) {
	// Log essential operational info at INFO level
	logFields := []any{
		"client_ip", pr.clientIP,
		"method", pr.method,
		"path", pr.path,
		"compatible_endpoints", endpointCount,
	}

	// Add user agent if present
	if pr.userAgent != "" {
		logFields = append(logFields, "user_agent", pr.userAgent)
	}

	// Add model if identified
	if pr.model != "" {
		logFields = append(logFields, "model", pr.model)
	}

	// Add content length if it's a POST/PUT with body
	if pr.contentLength > 0 {
		logFields = append(logFields, "content_length", pr.contentLength)
	}

	// translator_mode is only set on translation handler requests
	if pr.translatorMode != "" {
		logFields = append(logFields, "translator_mode", string(pr.translatorMode))
	}

	// A request with no routable endpoint is about to be logged by logRequestRejected, which
	// carries the same identifying fields and is sampled. Writing Info here as well would
	// leave an unthrottled line per request for a client repeating an unknown model.
	if endpointCount == 0 {
		pr.requestLogger.Debug("Request received", logFields...)
	} else {
		pr.requestLogger.Info("Request received", logFields...)
	}

	// Log additional details at DEBUG level
	debugFields := []any{
		"target_path", pr.targetPath,
		"path_resolution_ms", pr.stats.PathResolutionMs,
		"query", pr.query,
		"content_type", pr.contentType,
	}

	pr.requestLogger.Debug("Request details", debugFields...)
}

// logRequestRejected records a request that never reached the proxy engine because
// endpoint selection produced no routable target. Kept distinct from logRequestResult's
// "completed"/"failed" outcomes so a fail-fast rejection (e.g. strict model_not_found)
// is surfaced as a rejection rather than a successful completion (#191).
func (a *Application) logRequestRejected(pr *proxyRequest, status int, reason string) {
	if !a.rejectLog.allowRequest(pr.clientIP, status) {
		return
	}
	duration := time.Since(pr.stats.StartTime)

	logFields := []any{
		"client_ip", pr.clientIP,
		"path", pr.path,
		"status", status,
		"reason", reason,
		"duration_ms", duration.Milliseconds(),
	}

	if pr.model != "" {
		logFields = append(logFields, "model", pr.model)
	}

	// routing fields explain why nothing was routable (strategy/action/reason)
	if rd := pr.stats.RoutingDecision; rd != nil {
		if rd.Strategy != "" {
			logFields = append(logFields, "routing_strategy", rd.Strategy)
		}
		if rd.Action != "" {
			logFields = append(logFields, "routing_action", rd.Action)
		}
		if rd.Reason != "" {
			logFields = append(logFields, "routing_reason", rd.Reason)
		}
	}

	pr.requestLogger.Warn("Request rejected", logFields...)
}

func (a *Application) logRequestResult(pr *proxyRequest, err error) {
	// A chunked body over the size cap fails mid-read and is answered 413 by handleProxyError.
	// It is a client rejection, so it takes the sampled path rather than an ERROR per request.
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		a.logRequestRejected(pr, http.StatusRequestEntityTooLarge, "request body too large")
		return
	}

	duration := time.Since(pr.stats.StartTime)

	if err != nil {
		logFields := a.buildLogFields(pr, duration)
		// What the client actually got, so a truncated 200 reads differently from a 502.
		if pr.upstreamStatus > 0 {
			logFields = append(logFields, "http_status", pr.upstreamStatus)
		}
		pr.requestLogger.Error("Request failed", append([]any{"error", err}, logFields...)...)
	} else {
		// Log essential completion info at INFO level
		infoFields := []any{
			"endpoint", pr.stats.EndpointName,
			"duration_ms", duration.Milliseconds(),
			"status", "completed",
		}

		if pr.upstreamStatus > 0 {
			infoFields = append(infoFields, "http_status", pr.upstreamStatus)
		}

		if pr.model != "" {
			infoFields = append(infoFields, "model", pr.model)
		}

		if pr.stats.TotalBytes > 0 {
			infoFields = append(infoFields, "total_bytes", pr.stats.TotalBytes)
		}

		// translator_mode is only set on translation handler requests
		if pr.translatorMode != "" {
			infoFields = append(infoFields, "translator_mode", string(pr.translatorMode))
		}

		infoFields = appendProviderMetricFields(infoFields, pr.stats.ProviderMetrics)

		// routing fields at INFO so operators can see the strategy without enabling DEBUG
		if rd := pr.stats.RoutingDecision; rd != nil {
			if rd.Strategy != "" {
				infoFields = append(infoFields, "routing_strategy", rd.Strategy)
			}
			if rd.Action != "" {
				infoFields = append(infoFields, "routing_action", rd.Action)
			}
			if rd.Reason != "" {
				infoFields = append(infoFields, "routing_reason", rd.Reason)
			}
		}

		// "disabled" carries no signal; omit it to avoid noise in deployments without sticky sessions
		if pr.stickyOutcome != "" && pr.stickyOutcome != "disabled" {
			infoFields = append(infoFields, "sticky_outcome", pr.stickyOutcome)
		}

		// the client-supplied session id, logged alongside the outcome so affinity
		// routing can be traced without enabling DEBUG
		if pr.sessionID != "" {
			infoFields = append(infoFields, "session_id", pr.sessionID)
		}

		// FallbackReasonNone is the empty string (passthrough succeeded); omit it
		// because it would appear on the majority of requests and adds no signal
		if pr.translatorFallbackReason != "" {
			infoFields = append(infoFields, "fallback_reason", pr.translatorFallbackReason)
		}

		// A backend 5xx is a backend fault the operator should see when filtering on WARN.
		// It is per request by design: unlike Olla's own rejections (sampled by
		// logRequestRejected) this is the only record of what the backend said. Backend 4xx
		// is the caller's problem and stays at Info.
		if pr.upstreamStatus >= http.StatusInternalServerError {
			pr.requestLogger.Warn("Request completed", infoFields...)
		} else {
			pr.requestLogger.Info("Request completed", infoFields...)
		}

		// Log detailed metrics at DEBUG level
		debugFields := a.buildLogFields(pr, duration)
		pr.requestLogger.Debug("Request metrics", debugFields...)
	}
}

// appendProviderMetricFields adds the backend-reported usage fields that are worth an INFO line.
func appendProviderMetricFields(fields []any, pm *domain.ProviderMetrics) []any {
	if pm == nil {
		return fields
	}
	if pm.InputTokens > 0 {
		fields = append(fields, "input_tokens", pm.InputTokens)
	}
	if pm.OutputTokens > 0 {
		fields = append(fields, "output_tokens", pm.OutputTokens)
	}
	if pm.TotalTokens > 0 {
		fields = append(fields, "total_tokens", pm.TotalTokens)
	}
	if pm.TokensPerSecond > 0 {
		fields = append(fields, "tokens_per_sec", fmt.Sprintf("%.1f", pm.TokensPerSecond))
	}
	if pm.TTFTMs > 0 {
		fields = append(fields, "ttft_ms", pm.TTFTMs)
	}
	// the model the backend actually ran, which can differ from the requested alias
	if pm.Model != "" {
		fields = append(fields, "provider_model", pm.Model)
	}
	return fields
}

func (a *Application) buildLogFields(pr *proxyRequest, duration time.Duration) []any {
	fields := []any{
		"endpoint", pr.stats.EndpointName,
		"model", pr.model,
		"client_ip", pr.clientIP,
		"total_bytes", pr.stats.TotalBytes,
		"duration_ms", duration.Milliseconds(),
		"latency_ms", pr.stats.Latency,
		"request_processing_ms", pr.stats.RequestProcessingMs,
		"backend_response_ms", pr.stats.BackendResponseMs,
		"first_data_ms", pr.stats.FirstDataMs,
		"streaming_ms", pr.stats.StreamingMs,
		"header_processing_ms", pr.stats.HeaderProcessingMs,
		"path_resolution_ms", pr.stats.PathResolutionMs,
		"selection_ms", pr.stats.SelectionMs,
	}

	if pr.stats.EndpointName == "" {
		fields = append(fields, "target_path", pr.targetPath)
	}

	// "none" means no sticky session is active, which adds no useful signal at DEBUG
	if pr.stickySource != "" && pr.stickySource != "none" {
		fields = append(fields, "sticky_source", pr.stickySource)
	}
	// also carried here so failed requests and the detailed metrics line keep
	// the session id (the success path logs it at INFO)
	if pr.sessionID != "" {
		fields = append(fields, "session_id", pr.sessionID)
	}

	// Add provider metrics if available (detailed view)
	if pr.stats.ProviderMetrics != nil {
		pm := pr.stats.ProviderMetrics
		if pm.InputTokens > 0 {
			fields = append(fields, "input_tokens", pm.InputTokens)
		}
		if pm.OutputTokens > 0 {
			fields = append(fields, "output_tokens", pm.OutputTokens)
		}
		if pm.TotalTokens > 0 {
			fields = append(fields, "total_tokens", pm.TotalTokens)
		}
		if pm.TokensPerSecond > 0 {
			fields = append(fields, "tokens_per_sec", pm.TokensPerSecond)
		}
		if pm.TTFTMs > 0 {
			fields = append(fields, "ttft_ms", pm.TTFTMs)
		}
		if pm.PromptMs > 0 {
			fields = append(fields, "prompt_ms", pm.PromptMs)
		}
		if pm.GenerationMs > 0 {
			fields = append(fields, "generation_ms", pm.GenerationMs)
		}
		if pm.Model != "" {
			fields = append(fields, "provider_model", pm.Model)
		}
		if pm.FinishReason != "" {
			fields = append(fields, "finish_reason", pm.FinishReason)
		}
	}

	return fields
}

func (a *Application) handleEndpointError(w http.ResponseWriter, pr *proxyRequest, err error) {
	pr.requestLogger.Error("Failed to get endpoints", "error", err)
	http.Error(w, fmt.Sprintf("Service unavailable: %v", err), http.StatusBadGateway)
}

// only send error response if we haven't started streaming yet.
// w.Started() is the real "response already committed" signal: Header().Set does
// not commit anything, only WriteHeader/Write do, so a Content-Type-presence check
// used to let this guard misfire on any handler that set headers before failing.
// This still prevents double-writing a response after a partial stream
// (learned this the hard way when users got html error messages appended to their json).
func (a *Application) handleProxyError(w *core.ResponseStartedWriter, err error) {
	if w.Started() {
		return
	}

	// A chunked (no Content-Length) request body that exceeds the configured
	// cap is caught mid-read by the http.MaxBytesReader SizeValidator.CreateMiddleware
	// installs on r.Body, not by the pre-check that handles known-Content-Length
	// oversize requests. That surfaces here as a generic read error wrapped up
	// through retry.go's body preservation - without this check it would fall
	// through to a misleading 502, when the real cause is the same "body too
	// large" condition the pre-check rejects with 413. Match that response
	// exactly so the client sees one consistent envelope regardless of which
	// path caught the oversize body.
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	http.Error(w, fmt.Sprintf("Proxy error: %v", err), http.StatusBadGateway)
}

// logNoCompatibleFallback records the widen-to-all fallback; translator routes expect it for a
// chat-less fleet and log it at Debug (see RequestProfile.QuietNoMatchFallback).
func logNoCompatibleFallback(log logger.StyledLogger, profile *domain.RequestProfile, total int) {
	logFn := log.Warn
	if profile.QuietNoMatchFallback {
		logFn = log.Debug
	}
	logFn("No compatible endpoints found for path, falling back to all endpoints",
		"path", profile.Path,
		"supported_by", profile.SupportedBy,
		"total_endpoints", total)
}

func (a *Application) stripRoutePrefix(ctx context.Context, path string) string {
	return util.StripRoutePrefix(ctx, path, constants.ContextRoutePrefixKey)
}

// three-stage filtering pipeline that progressively narrows down endpoints.
// starts broad (platform compatibility), then capabilities (vision, embeddings),
// finally specific model availability. each stage falls back gracefully.
func (a *Application) filterEndpointsByProfile(endpoints []*domain.Endpoint, profile *domain.RequestProfile, logger logger.StyledLogger) []*domain.Endpoint {
	var profileFiltered []*domain.Endpoint

	// stage 1: platform compatibility (ollama can't handle openai requests etc)
	if profile == nil || len(profile.SupportedBy) == 0 {
		logger.Debug("No profile filtering applied", "total_endpoints", len(endpoints))
		profileFiltered = endpoints
	} else {
		compatible := make([]*domain.Endpoint, 0, len(endpoints))
		for _, endpoint := range endpoints {
			// Normalise endpoint type to handle variations (e.g., lmstudio -> lm-studio)
			normalizedType := NormaliseProviderType(endpoint.Type)
			if profile.IsCompatibleWith(normalizedType) {
				compatible = append(compatible, endpoint)
			}
		}

		if len(compatible) == 0 {
			if profile.FailClosedOnNoMatch {
				// Provider-scoped request (e.g. /olla/vllm/) with zero healthy
				// endpoints of that type: return the empty set so the caller's
				// existing no-routable-endpoints path produces a proper 404/503,
				// instead of silently crossing into another provider type.
				// Debug, not Warn: a client can repeat this at wire speed. The sampled
				// "Request rejected" line carries the cause ("No <type> endpoints available").
				logger.Debug("No compatible endpoints found for provider-scoped path, failing closed",
					"path", profile.Path,
					"supported_by", profile.SupportedBy,
					"total_endpoints", len(endpoints))
				return nil
			}
			logNoCompatibleFallback(logger, profile, len(endpoints))
			profileFiltered = endpoints
		} else {
			logger.Debug("Filtered endpoints by profile compatibility",
				"path", profile.Path,
				"compatible_count", len(compatible),
				"total_count", len(endpoints),
				"supported_by", profile.SupportedBy)
			profileFiltered = compatible
		}
	}

	// stage 2: capability filtering (vision requests need vision models).
	// only for model-less requests: capability tags are heuristic, so when the
	// caller names a model, stage 3 alone decides which endpoints host it. a
	// tag on some other model must never evict the hosts of the requested one.
	if profile != nil && profile.ModelName == "" && profile.ModelCapabilities != nil && a.modelRegistry != nil {
		capabilityFiltered, refusal := a.filterEndpointsByCapabilities(profileFiltered, profile, logger)
		if refusal != nil {
			// empty result makes dispatchToEndpoints answer with the decision's 503
			profile.RoutingDecision = refusal
			return []*domain.Endpoint{}
