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
