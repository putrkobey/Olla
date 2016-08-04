package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/util"

	"github.com/putrkobey/Olla/internal/logger"
)

// Context keys for request ID and logger
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	LoggerKey    contextKey = "logger"
)

const (
	// maxRequestIDLength caps the inbound X-Request-ID length. Values beyond this
	// are most likely probing attempts or misconfigured clients; generating a fresh
	// ID is cheaper than propagating an unbounded string into every log line.
	maxRequestIDLength = 128
)

// sanitiseRequestID validates a client-supplied request ID and returns it
// unchanged if it passes. An empty string signals the caller to generate a
// fresh ID instead. Rejected when: longer than maxRequestIDLength, or contains
// any character that is not a printable ASCII non-space (CR, LF, NUL, tabs and
// other control characters are log-injection vectors).
func sanitiseRequestID(id string) string {
	if len(id) > maxRequestIDLength {
		return ""
	}
	for _, c := range id {
		// Only allow printable ASCII (0x21–0x7E). Space (0x20) is technically
		// printable but rarely intentional in IDs and trips some log parsers.
		if c < 0x21 || c > 0x7E {
			return ""
		}
	}
	return id
}

// IsProxyRequest determines if a request is for the proxy endpoints
// Used to decide logging levels to avoid redundancy with proxy handler logging
func IsProxyRequest(path string) bool {
	// checks for proxy prefixes
	// /olla/ is the main proxy prefix
	return strings.Contains(path, constants.DefaultOllaProxyPathPrefix) ||
		(strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/api/v0/")) // /api/v0/ is internal
}

// internalPathPrefix is where the admin dashboard's status/health/stats polls
// land. An open dashboard tab polls several of these every few seconds; at
// info level that floods both the console log and the dedicated access log
// with traffic that carries no operational signal, so successful polling is
// demoted to debug alongside the existing proxy hot-path treatment. A 404,
// wrong-method request, 5xx, or repeated 403 under /internal/ (e.g. the
// dashboard's access-control gate being probed) must never be swallowed by
// this, so quieting is conditioned on outcome, not just path — see
// isQuietPollOutcome.
const internalPathPrefix = "/internal/"

// isInternalPollMethod reports whether method is one a polling GET/HEAD
// client would use. Anything else (POST, PUT, DELETE, ...) against
// /internal/ is never routine polling traffic and must always log normally.
func isInternalPollMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead
}

// isQuietPollRoute reports whether path/method belongs to a route whose
// traffic is presumptively routine and high-frequency, for the PRE-request
// log line where the response status does not exist yet: the proxy hot path
// (already unconditionally demoted) plus a GET/HEAD request under /internal/.
// This is deliberately optimistic — chosen approach (b): quiet the
// pre-request line by method/path alone, and let the post-request line
// (isQuietPollOutcome) apply the real, status-aware gate. The alternative,
// deferring the pre-request decision entirely to the post-request call,
// would mean either logging every dashboard poll's "started" line at Info
// (reintroducing the flood this exists to prevent) or logging it at Debug
// unconditionally including for non-GET/HEAD internal requests, which is a
// worse trade for a line that carries no status/duration/diagnostic value
// anyway. The "completed" line below is the one that must never hide a
// real problem, and it does not depend on this pre-request line's decision.
func isQuietPollRoute(method, path string) bool {
	return IsProxyRequest(path) || (isInternalPollMethod(method) && strings.HasPrefix(path, internalPathPrefix))
}

// isQuietPollOutcome is the authoritative, status-aware gate applied once
// the response is known: the POST-request log line and the single access-log
// line both use this, not isQuietPollRoute. A /internal/ request is only
// treated as quiet polling traffic when it is GET/HEAD AND the response was
// 2xx or 304 — anything else (404, 4xx, 5xx, or a non-GET/HEAD method that
// happened to succeed) logs at its normal level regardless of path, so a
// probed dashboard access-control 403, a wrong-route 404, or a 500 is never
// invisible at the default Info level just because it lives under /internal/.
func isQuietPollOutcome(method, path string, status int) bool {
	if IsProxyRequest(path) {
		return true
	}
	if !isInternalPollMethod(method) || !strings.HasPrefix(path, internalPathPrefix) {
		return false
	}
	return (status >= 200 && status < 300) || status == http.StatusNotModified
}

// isQuietAccessOutcome is the access-log analogue of isQuietPollOutcome.
// The access log is the operator's audit record of every request that hit
// Olla, so unlike the console path it must NOT quiet proxy traffic: a proxy
// success is the audit-worthy outcome the security-practices doc promises to
// record at the default Info level, and a proxy 4xx/5xx must never disappear
// into Debug. Only the dashboard's own /internal/ polling surface is
// routine enough to demote, and even then only for GET/HEAD + 2xx/304:
// a 404, 403, 5xx, or non-GET/HEAD method under /internal/ stays loud so a
// probed access-control gate or failing handler is visible at Info. The
// console path (isQuietPollOutcome) intentionally still quiets proxy traffic
// regardless of status because the console has dedicated proxy-handler
// logging; do not unify the two.
func isQuietAccessOutcome(method, path string, status int) bool {
	if !isInternalPollMethod(method) || !strings.HasPrefix(path, internalPathPrefix) {
		return false
	}
	return (status >= 200 && status < 300) || status == http.StatusNotModified
}

// accessLogLevel resolves the slog level the access log should emit at for a
// given outcome, applying the status-aware isQuietAccessOutcome gate. Factored
// out as a pure function so the level table is testable directly without a
// capturing slog handler.
func accessLogLevel(method, path string, status int) slog.Level {
	if isQuietAccessOutcome(method, path, status) {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// consoleLogParams resolves the level and message the console log lines
// (request started/completed) should use for a given quiet gate result.
