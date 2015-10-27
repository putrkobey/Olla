package security

/*
				Olla Security Adapter - Size Limit Validator
	SizeValidator enforces request size limits for headers and body content.
 	It checks these limits early in the middleware chain to avoid wasting resources
 	on oversized requests.

	Thread-safe by design as it maintains no internal mutable state.
*/

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/util"
)

const (
	DefaultProtocol = "HTTP/1.1"

	// minHeaderSize floors a configured max_header_size so a small misconfigured
	// value can't cripple every request's headers. Mirrors the floor
	// services.minMaxHeaderBytes applies to the stdlib http.Server layer.
	minHeaderSize = 4096
)

type SizeValidator struct {
	logger        logger.StyledLogger
	metrics       ports.SecurityMetricsService
	rejectLog     *logger.Throttle
	maxBodySize   int64
	maxHeaderSize int64
}

func NewSizeValidator(limits config.ServerRequestLimits, metrics ports.SecurityMetricsService, log logger.StyledLogger) *SizeValidator {
	sv := &SizeValidator{
		maxBodySize:   limits.MaxBodySize,
		maxHeaderSize: effectiveMaxHeaderSize(limits.MaxHeaderSize),
		metrics:       metrics,
		logger:        log,
	}
	// Oversized requests can be sent at wire speed, so the WARN is sampled per
	// client IP; the violation counters still see every rejection.
	sv.rejectLog = logger.NewRejectThrottle(log, "Request rejected", "size_limit", "client_kind", nil)
	return sv
}

// Flush writes any pending rejection summary. The validator has no goroutines to
// stop, so shutdown only needs this to report the tail of a flood.
func (sv *SizeValidator) Flush() {
	sv.rejectLog.Flush()
}

// effectiveMaxHeaderSize derives the enforced header cap from the configured
// max_header_size. A zero or negative value is "unset", not "disabled" - it
// falls back to http.DefaultMaxHeaderBytes (1 MiB), matching the stdlib
// http.Server fallback in services.maxHeaderBytesFromConfig so the app-level
// policy check and the listener-level hard cap agree by default. A small
// positive value is floored at minHeaderSize so a misconfiguration can't
// reject every legitimate request.
func effectiveMaxHeaderSize(configured int64) int64 {
	if configured <= 0 {
		return http.DefaultMaxHeaderBytes
	}
	if configured < minHeaderSize {
		return minHeaderSize
	}
	return configured
}

func (sv *SizeValidator) Name() string {
	return "size_limit"
}

// Validate checks the request against configured size constraints.
// Returns a SecurityResult indicating whether the request is allowed.
func (sv *SizeValidator) Validate(ctx context.Context, req ports.SecurityRequest) (ports.SecurityResult, error) {
	if err := sv.validateHeaderSize(req); err != nil {
		return ports.SecurityResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Request headers too large: %v", err),
		}, nil
	}

	if err := sv.validateBodySize(req); err != nil {
		return ports.SecurityResult{
			Allowed: false,
			Reason:  fmt.Sprintf("Request body too large: %v", err),
		}, nil
	}

	return ports.SecurityResult{
		Allowed: true,
	}, nil
}

// validateHeaderSize estimates total header size, including field names and values.
// Returns an error if the combined size exceeds the configured max.
func (sv *SizeValidator) validateHeaderSize(req ports.SecurityRequest) error {
	totalSize := estimateHeaderSize(req.Headers, req.Method, req.Endpoint, DefaultProtocol) // assume HTTP/1.1
	if totalSize > sv.maxHeaderSize {
		return fmt.Errorf("header size %d exceeds limit %d", totalSize, sv.maxHeaderSize)
	}
	return nil
}

// validateBodySize checks the request body size against the configured limit.
func (sv *SizeValidator) validateBodySize(req ports.SecurityRequest) error {
	if sv.maxBodySize <= 0 {
		return nil
	}

	if req.BodySize > sv.maxBodySize {
		return fmt.Errorf("content-length %d exceeds limit %d", req.BodySize, sv.maxBodySize)
	}

	return nil
}

// CreateMiddleware enforces the size limits on streaming (proxy) routes, capping the body
// with MaxBytesReader rather than buffering it. reject writes the rejection for routes whose
// clients expect a specific error format (see RejectFunc); nil keeps the plain-text default.
func (sv *SizeValidator) CreateMiddleware(reject RejectFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := ports.SecurityRequest{
				Endpoint:   r.URL.Path,
				Method:     r.Method,
				BodySize:   r.ContentLength,
				HeaderSize: estimateHeaderSize(r.Header, r.Method, r.URL.RequestURI(), r.Proto),
				Headers:    r.Header,
			}

			result, err := sv.Validate(r.Context(), req)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !result.Allowed {
				sv.recordViolation(r, req, result)

				if r.ContentLength > sv.maxBodySize {
					writeReject(w, reject, http.StatusRequestEntityTooLarge, "Request body too large")
				} else {
					writeReject(w, reject, http.StatusRequestHeaderFieldsTooLarge, "Request headers too large")
				}
				return
			}

			if sv.maxBodySize > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, sv.maxBodySize)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CreateNonProxyMiddleware is like CreateMiddleware but also rejects chunked
// (Content-Length == -1) requests whose bodies exceed the configured limit.
// It is intended for non-proxy routes (status, health, stats) that never read
// the body themselves, so MaxBytesReader alone would never fire for them.
//
// The approach: drain up to maxBodySize+1 bytes through MaxBytesReader before
// calling next. If the read hits the limit the reader returns an error and we
// return 413. Otherwise we restore the buffered bytes as the request body so
// a handler that does read the body still receives the full content.
//
// Proxy routes must NOT use this middleware — they stream large bodies
// legitimately and must not be buffered. Use CreateMiddleware for those paths.
// Translator routes do use it: their handler buffers the whole body anyway, and this
// keeps an oversized chunked body a counted 413 instead of a failed read later on.
func (sv *SizeValidator) CreateNonProxyMiddleware(reject RejectFunc) func(http.Handler) http.Handler {
	return sv.nonProxyMiddleware(reject, 0)
}

// CreateCappedNonProxyMiddleware is CreateNonProxyMiddleware with a tighter drain cap for
// chunked bodies: min(max_body_size, drainCap). A route whose handler enforces a smaller limit
// of its own (translators) would otherwise buffer up to max_body_size per chunked request
// before that limit rejects it. Content-Length requests are still checked against
// max_body_size only, as before; drainCap <= 0 means no extra cap.
func (sv *SizeValidator) CreateCappedNonProxyMiddleware(reject RejectFunc, drainCap int64) func(http.Handler) http.Handler {
	return sv.nonProxyMiddleware(reject, drainCap)
}

func (sv *SizeValidator) nonProxyMiddleware(reject RejectFunc, drainCap int64) func(http.Handler) http.Handler {
	chunkedLimit := sv.maxBodySize
	if drainCap > 0 && (chunkedLimit <= 0 || drainCap < chunkedLimit) {
		chunkedLimit = drainCap
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			req := ports.SecurityRequest{
				Endpoint:   r.URL.Path,
				Method:     r.Method,
				BodySize:   r.ContentLength,
				HeaderSize: estimateHeaderSize(r.Header, r.Method, r.URL.RequestURI(), r.Proto),
				Headers:    r.Header,
			}
