package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/util"
	"github.com/tidwall/gjson"
)

// executePassthroughRequest handles requests that can be forwarded directly to backends
// without translation (e.g. Anthropic API requests to vLLM with native Anthropic support).
// bodyBytes is the pre-buffered request body from the handler, passed through to avoid re-reading.
func (a *Application) executePassthroughRequest(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	bodyBytes []byte,
	endpoints []*domain.Endpoint,
	pr *proxyRequest,
	trans translator.RequestTranslator,
) {
	// Get passthrough request details
	passthroughTrans, ok := trans.(translator.PassthroughCapable)
	if !ok {
		// This should never happen since we checked the interface before calling this function
		a.writeTranslatorError(w, trans, pr, errors.New("translator does not support passthrough"), http.StatusInternalServerError)
		return
	}

	passthroughReq, err := passthroughTrans.PreparePassthrough(bodyBytes, r, a.profileLookup)
	if err != nil {
		a.writeTranslatorError(w, trans, pr, err, http.StatusBadRequest)
		return
	}

	// The proxy selector chooses the actual endpoint later. When capable backends
	// disagree on their native Anthropic path (e.g. DMR uses /anthropic/v1/messages
	// while vLLM uses /v1/messages), we restrict to the largest path-compatible
	// subset so the selected backend and the request path always agree.
	resolvedPath, filteredEndpoints := a.resolvePassthroughTargetPath(endpoints, passthroughReq.TargetPath)
	if resolvedPath != "" {
		passthroughReq.TargetPath = resolvedPath
	}
	if len(filteredEndpoints) > 0 {
		endpoints = filteredEndpoints
	}

	// Update proxy request details - capture streaming flag for accurate metrics
	// (StreamingMs isn't populated in passthrough mode since we don't intercept the stream)
	pr.isStreaming = passthroughReq.IsStreaming

	// Set mode before logRequestStart so it appears on the lifecycle log lines.
	pr.translatorMode = constants.TranslatorModePassthrough

	pr.requestLogger.Debug("using passthrough mode (native Anthropic support)",
		"model", passthroughReq.ModelName,
		"streaming", passthroughReq.IsStreaming,
		"target_path", passthroughReq.TargetPath,
		"endpoints", len(endpoints))

	// Set request body and path
	r.Body = io.NopCloser(bytes.NewReader(passthroughReq.Body))
	r.ContentLength = int64(len(passthroughReq.Body))
	r.URL.Path = passthroughReq.TargetPath

	// Add passthrough mode header for observability
	w.Header().Set(constants.HeaderXOllaMode, string(constants.TranslatorModePassthrough))

	// Prepare context
	ctx, r = a.prepareProxyContext(ctx, r, pr)

	// Log request start
	a.logRequestStart(pr, len(endpoints))

	// Execute proxy
	tracker := core.NewResponseStartedWriter(w)
	err = a.proxyService.ProxyRequestToEndpoints(ctx, tracker, r, endpoints, pr.stats, pr.requestLogger)

	// Error response first, through the tracker, so the logged status is what the client got.
	if err != nil {
		// only write error if response hasn't started
		if w.Header().Get(constants.HeaderContentType) == "" {
			a.writeTranslatorError(tracker, trans, pr, fmt.Errorf("proxy error: %w", err), http.StatusBadGateway)
		}
	}
	pr.upstreamStatus = tracker.Status()
	pr.captureStickyOutcome(ctx, r)
	a.logRequestResult(pr, err)

	pr.stats.EndTime = time.Now()
}

// resolvePassthroughTargetPath resolves the native Anthropic messages path and
// the endpoint subset that must be used with it.
//
// Uniform fleets: all candidates share the same MessagesPath. Returns that path
// and nil (caller keeps original endpoint list unchanged).
//
// Mixed fleets: candidates disagree on their native path (e.g. DMR uses
// /anthropic/v1/messages while vLLM uses /v1/messages). Restricting the proxy
// to the full list would cause a 404 on whichever backend is selected but whose
// path was not used. Instead we return the largest path-compatible subset.
// Tie-break: prefer the subset whose path equals defaultPath; if no subset
// matches, use the path whose first endpoint appears earliest in the original
// endpoints slice. This ensures deterministic selection regardless of map
// iteration order.
//
// If profileLookup is nil or the endpoint list is empty, returns defaultPath and
// nil so the caller leaves everything unchanged.
func (a *Application) resolvePassthroughTargetPath(endpoints []*domain.Endpoint, defaultPath string) (string, []*domain.Endpoint) {
	if a.profileLookup == nil || len(endpoints) == 0 {
		return defaultPath, nil
	}

	// Build path→endpoints groups while recording the first-seen index for each
	// path. Iterating the original slice preserves endpoint order so the
	// first-seen index is stable across calls.
	pathGroups := make(map[string][]*domain.Endpoint, len(endpoints))
	firstSeen := make(map[string]int, len(endpoints)) // path → index of first endpoint in slice
	for i, ep := range endpoints {
		support := a.profileLookup.GetAnthropicSupport(ep.Type)
		path := defaultPath
		if support != nil && support.MessagesPath != "" {
			path = support.MessagesPath
		}
		if _, exists := firstSeen[path]; !exists {
			firstSeen[path] = i
		}
		pathGroups[path] = append(pathGroups[path], ep)
	}

	// Uniform fleet: all candidates agree on a single path.
	if len(pathGroups) == 1 {
		for path := range pathGroups {
			// nil filtered list signals "keep original slice" to the caller.
			return path, nil
		}
	}

	// Mixed fleet: pick the largest subset. Tie-break order:
	//   1. defaultPath wins (avoids surprising path changes for the majority case).
	//   2. The path whose first endpoint appears earliest in the original slice
	//      wins (deterministic, not dependent on map iteration order).
	var bestPath string
	var bestCount int
	bestFirstSeen := -1

	for path, group := range pathGroups {
		count := len(group)
		fs := firstSeen[path]
		switch {
		case count > bestCount:
			bestPath, bestCount, bestFirstSeen = path, count, fs
		case count == bestCount && path == defaultPath:
			// defaultPath takes priority in a tie regardless of first-seen.
			bestPath, bestFirstSeen = path, fs
		case count == bestCount && bestPath != defaultPath && fs < bestFirstSeen:
			// Neither candidate is defaultPath; earliest first-seen endpoint wins.
			bestPath, bestFirstSeen = path, fs
		}
	}

	return bestPath, pathGroups[bestPath]
}

// executeTranslationRequest handles the translation path where requests are converted
// from the translator's native format (e.g. Anthropic) to OpenAI format for the backend
func (a *Application) executeTranslationRequest(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	endpoints []*domain.Endpoint,
	pr *proxyRequest,
	trans translator.RequestTranslator,
	transformedReq *translator.TransformedRequest,
) constants.TranslatorFallbackReason {
	// Capture streaming flag for metrics before proxying
	pr.isStreaming = transformedReq.IsStreaming

	// Set mode before logRequestStart so it appears on the lifecycle log lines.
	pr.translatorMode = constants.TranslatorModeTranslation

	// Serialize OpenAI request
	openaiBody, err := json.Marshal(transformedReq.OpenAIRequest)
	if err != nil {
		a.writeTranslatorError(w, trans, pr, errors.New("failed to serialize request"), http.StatusInternalServerError)
		return ""
	}

	r.Body = io.NopCloser(bytes.NewReader(openaiBody))
	r.ContentLength = int64(len(openaiBody))

	// Handle path translation if specified
	if transformedReq.TargetPath != "" {
		targetPath := util.StripPrefix(transformedReq.TargetPath, constants.DefaultOllaProxyPathPrefix)

		if targetPath != transformedReq.TargetPath {
			pr.requestLogger.Warn("TargetPath included proxy prefix, stripped it",
				"translator", trans.Name(),
				"proxy_prefix", constants.DefaultOllaProxyPathPrefix,
				"original_target", transformedReq.TargetPath,
				"corrected_target", targetPath)
		}

		pr.requestLogger.Debug("Path translation applied",
			"original_path", r.URL.Path,
			"target_path", targetPath,
			"translator", trans.Name())
		r.URL.Path = targetPath
	} else if trans.Name() != "passthrough" {
		// warn if translator might need path translation (passthrough can ignore)
		pr.requestLogger.Warn("Translator did not set TargetPath, using original path",
			"translator", trans.Name(),
			"original_path", r.URL.Path,
			"note", "This may cause routing issues if translation requires different endpoint")
	}

	// Resolve the chat path each candidate actually serves. The default r.URL.Path above is
	// only right for backends that mount the OpenAI API at the root.
	var usable []*domain.Endpoint
	usable, pr.endpointPaths = a.translationEndpointPaths(endpoints, r.URL.Path)
	if len(usable) == 0 {
		// A routing rejection like the no-endpoints one in the caller: sampled log line,
		// quiet error write, and the no-compatible-endpoints reason in the metrics.
		reason := fmt.Sprintf("no selected endpoint serves %s", r.URL.Path)
		pr.translatorFallbackReason = string(constants.FallbackReasonNoCompatibleEndpoints)
		a.setStickyResponseHeadersFromRequest(w, r)
		a.logRequestStart(pr, 0)
		a.logRequestRejected(pr, http.StatusNotFound, reason)
		a.respondTranslatorError(w, trans, pr, errors.New(reason), http.StatusNotFound, true)
		return constants.FallbackReasonNoCompatibleEndpoints
	}
	endpoints = usable

	a.logRequestStart(pr, len(endpoints))

	// Observe the status the client receives so a backend's own 4xx/5xx is logged.
	tracker := core.NewResponseStartedWriter(w)
	w = tracker

	// For streaming requests, seed the context with an estimated input token count
	// computed from the original (pre-translation) request body. This lets the
	// streaming translator populate a non-zero input_tokens in message_start, matching
	// the behaviour of vLLM and lmdeploy which both emit real input_tokens at that point.
	// The estimate is overwritten by the actual upstream usage when it arrives.
	if transformedReq.IsStreaming {
		if estimator, ok := trans.(translator.InputTokenEstimator); ok && len(transformedReq.OriginalBody) > 0 {
			estimate := estimator.EstimateInputTokens(transformedReq.OriginalBody)
			ctx = context.WithValue(ctx, constants.ContextInputTokensKey, estimate)
			r = r.WithContext(ctx)
		}
	}

	// Execute proxy with appropriate response handling (streaming vs non-streaming)
	var proxyErr error
	if transformedReq.IsStreaming {
		proxyErr = a.executeTranslatedStreamingRequest(ctx, w, r, endpoints, pr, trans)
	} else {
		proxyErr = a.executeTranslatedNonStreamingRequest(ctx, w, r, endpoints, pr, trans)
	}

	if proxyErr != nil {
		// only write error if response hasn't started
		if w.Header().Get(constants.HeaderContentType) == "" {
			a.writeTranslatorError(w, trans, pr, fmt.Errorf("proxy error: %w", proxyErr), http.StatusBadGateway)
		}
	}
	pr.upstreamStatus = tracker.Status()
	pr.captureStickyOutcome(ctx, r)
	a.logRequestResult(pr, proxyErr)

	pr.stats.EndTime = time.Now()
	return ""
}

// translationEndpointPaths pins each candidate to the path it serves for the translator's
// target path, and drops candidates that serve none. The translator rewrites every request
// to one OpenAI path, but DMR mounts it at /engines/v1/chat/completions and Lemonade at
// /api/v1/chat/completions, so sending the fixed path 404s there. Returns the usable
// endpoints and, when any of them needs a path other than targetPath, the per-endpoint map.
//
// Without a path inspector (no profiles loaded) nothing can be resolved and the list is
// returned untouched.
func (a *Application) translationEndpointPaths(endpoints []*domain.Endpoint, targetPath string) ([]*domain.Endpoint, map[string]string) {
	if a.pathInspector == nil || targetPath == "" {
		return endpoints, nil
	}

	usable := make([]*domain.Endpoint, 0, len(endpoints))
	paths := make(map[string]string, len(endpoints))
	differs := false
	for _, ep := range endpoints {
		profileName := NormaliseProviderType(ep.Type)
		resolved, ok := a.pathInspector.ResolveTargetPath(profileName, targetPath)
		if !ok {
			continue
		}
		resolved = a.relativeToPreservedBase(ep, profileName, targetPath, resolved)
		usable = append(usable, ep)
		paths[ep.GetURLString()] = resolved
		if resolved != targetPath {
			differs = true
		}
	}
	if !differs {
		return usable, nil
	}
	return usable, paths
}

// relativeToPreservedBase adjusts a resolved chat path for a preserve_path endpoint. The proxy
// joins the endpoint's base onto the request path, so when the base already ends in one of the
// profile's mounts (Lemonade at http://host/api, DMR at http://host/gw/engines/llama.cpp) the
// plain target is what must be sent; the resolved path would repeat the mount and 404. This is
// what main sent. A base with no mount in it (a gateway prefix such as /gw) keeps the resolved
// path, which is the mount the profile declares first.
func (a *Application) relativeToPreservedBase(ep *domain.Endpoint, profileName, target, resolved string) string {
	if !ep.PreservePath || ep.URL == nil {
		return resolved
	}
	base := strings.TrimRight(ep.URL.Path, "/")
	if base == "" {
		return resolved
	}
	for _, mount := range a.pathInspector.MountPrefixes(profileName, target) {
		if base == mount || strings.HasSuffix(base, mount) {
			return target
		}
	}
	return resolved
}

// scopeProfileToTranslation replaces the stage-1 compatibility set that analyzeRequest
// derived from the translator's own public route.
//
// Translator routes are registered without a prefix strip, so the inspector sees
// /olla/anthropic/v1/messages and suffix-matches it against /v1/messages, which only
// some profiles declare. Backends that speak Anthropic through anthropic_support (vLLM,
// llama.cpp, LM Studio) may not list that path, and Lemonade, LiteLLM and TEI never did,
// so the URL-derived set evicted them. What the request will actually hit decides who is
// compatible: passthrough sends it to backends with anthropic_support enabled,
// translation sends it to the translator's target path, wherever the profile mounts it.
//
// The anthropic_support profiles are added whether or not passthrough is enabled, because
// that is only known after endpoint selection. Where passthrough then declines,
// translationEndpointPaths drops any of them that serve no chat path, so the union cannot
// send a translated request somewhere it cannot land.
//
// Endpoint types with no profile of their own (custom types) are served as OpenAI-compatible
// by ResolveTargetPath, so they are added by name; otherwise stage 1 would evict them.
//
// An empty result leaves stage 1 permissive (all endpoints). When nothing matches (a TEI-only
// fleet) stage 1 still widens to all, as main does, so model routing keeps its own 503/404
// decisions; translationEndpointPaths then refuses with the sampled 404. The widening is
// logged at Debug here because it is the expected path for that fleet.
func (a *Application) scopeProfileToTranslation(ctx context.Context, pr *proxyRequest, trans translator.RequestTranslator) {
	if pr.profile == nil {
		return
	}

	supported := make([]string, 0, 8)

	// The per-endpoint path is resolved later (translationEndpointPaths), so a profile that
	// mounts the OpenAI API elsewhere is a candidate rather than being evicted here.
	if tp, ok := trans.(translator.TargetPathProvider); ok {
		if a.pathInspector == nil {
			// Without the profile paths the set would hold only the passthrough profiles and
			// evict every translation-only backend; stay permissive instead.
			pr.profile.SupportedBy = nil
			return
		}
		supported = append(supported, a.pathInspector.TranslationProfiles(tp.TranslationTargetPath())...)
	}

	if _, ok := trans.(translator.PassthroughCapable); ok && a.profileFactory != nil && a.profileLookup != nil {
		for _, name := range a.profileFactory.GetAvailableProfiles() {
			if support := a.profileLookup.GetAnthropicSupport(name); support != nil && support.Enabled {
				supported = append(supported, name)
			}
		}
	}

	if a.pathInspector != nil && a.discoveryService != nil {
		if endpoints, err := a.discoveryService.GetHealthyEndpoints(ctx); err == nil {
			for _, ep := range endpoints {
				if name := NormaliseProviderType(ep.Type); !a.pathInspector.KnowsProfile(name) {
					supported = append(supported, name)
				}
			}
		}
	}

	pr.profile.SupportedBy = pr.profile.SupportedBy[:0]
	for _, name := range supported {
		pr.profile.AddSupportedProfile(name)
	}
	pr.profile.QuietNoMatchFallback = true
}

// tryPassthrough attempts to serve the request via passthrough mode if the translator
// and at least one backend support it. Returns true if the request was handled.
func (a *Application) tryPassthrough(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	bodyBytes []byte,
	endpoints []*domain.Endpoint,
	pr *proxyRequest,
	trans translator.RequestTranslator,
) bool {
	passthroughTrans, ok := trans.(translator.PassthroughCapable)
	if !ok || a.profileLookup == nil {
		return false
	}

	// Only pass endpoints whose backend natively supports the wire format.
	// Mixed deployments (e.g. ollama + vllm) must not block passthrough for
	// the capable subset - the proxy will route within that filtered list.
	//
	// Additionally, exclude endpoints that declare a limitation matching a
	// feature present in this specific request. Token-counting limitations
	// (e.g. token_counting_404) use different names and are never affected.
	reqFeatures := detectRequestFeatures(bodyBytes)
	passthroughEndpoints := make([]*domain.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		support := a.profileLookup.GetAnthropicSupport(ep.Type)
		if support == nil || !support.Enabled {
			continue
		}
		if reqFeatures.toolUse && support.HasLimitation(domain.AnthropicLimitationNoToolUse) {
			continue
		}
		if reqFeatures.extendedThinking && support.HasLimitation(domain.AnthropicLimitationNoExtendedThinking) {
			continue
		}
		if reqFeatures.vision && support.HasLimitation(domain.AnthropicLimitationNoVision) {
			continue
		}
		passthroughEndpoints = append(passthroughEndpoints, ep)
	}

	if !passthroughTrans.CanPassthrough(passthroughEndpoints, a.profileLookup) {
		return false
	}

	a.executePassthroughRequest(ctx, w, r, bodyBytes, passthroughEndpoints, pr, trans)
	a.recordTranslatorMetrics(trans, pr, constants.TranslatorModePassthrough, constants.FallbackReasonNone)
	return true
}

// resolveTranslationFallback determines why passthrough was not used.
func (a *Application) resolveTranslationFallback(trans translator.RequestTranslator) constants.TranslatorFallbackReason {
	if _, ok := trans.(translator.PassthroughCapable); ok {
		return constants.FallbackReasonCannotPassthrough
	}
	return constants.FallbackReasonTranslatorDoesNotSupportPassthrough
}

// translatorBodyLimit is the request body cap for a translator. Translators that implement
// BodySizeLimiter declare their own max; others get a safe default. The size middleware uses
// the same value to bound chunked reads, so the two can't drift apart.
func translatorBodyLimit(trans translator.RequestTranslator) int64 {
	var limit int64 = 10 << 20 // 10 MiB default
	if limiter, ok := trans.(translator.BodySizeLimiter); ok {
		limit = limiter.MaxBodySize()
	}
	return limit
}

// generic handler for any translator (eg anthropic to openai and back)
func (a *Application) translationHandler(trans translator.RequestTranslator) http.HandlerFunc {
	// Resolve body size limit once at registration time, not per-request.
	maxBodySize := translatorBodyLimit(trans)

