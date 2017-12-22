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

