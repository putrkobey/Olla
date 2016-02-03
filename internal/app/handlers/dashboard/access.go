// Package dashboard wires the embedded admin dashboard into Olla's HTTP mux.
//
// This file holds the access-control middleware and the registration
// helper that mounts the dashboard subtree. The static-asset handler itself
// lives in embed.go.
//
// Security posture: there is no authentication. The dashboard relies entirely
// on the two network-layer checks here, both of which must pass per request:
//  1. the TCP source (r.RemoteAddr, never a proxy header) is inside an allowed
//     CIDR;
//  2. r.Host (port stripped, case-insensitive) is either an IP literal or
//     appears in allowed_hosts (IP-literal Hosts are always accepted because
//     DNS rebinding requires the browser to have resolved a hostname).
//
// A rejected request receives a self-diagnosing 403: the body states which
// check failed and names the rejected client IP and Host. There is no auth
// secret to protect by staying silent, and the operator is entitled to see
// the rejected IP/Host so the Docker first-run 403 doesn't resolve to "just
// set 0.0.0.0/0".
package dashboard

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
	"github.com/putrkobey/Olla/internal/util"
)

// DashboardRoute is the mux subtree the dashboard owns. Go's http.ServeMux
// treats the trailing slash as a catch-all, so this single registration covers
// /internal/ui/ and every sub-path. Mounted under /internal/ so its assets
// share the same access-policy zone as the data they display, and so no
// profile-driven provider route can collide (provider routes are always
// under /olla/).
const DashboardRoute = "/internal/ui/"

// SlashlessDashboardRoute is the exact-match path without the trailing slash.
// Go's ServeMux answers a request for this path with its own redirect to the
// trailing-slash subtree BEFORE any registered handler (and therefore before
// AccessMiddleware) ever runs, so a disallowed client previously got a bare
// redirect on this one path instead of the self-diagnosing 403 every other
// dashboard path returns - the policy still held on the canonical path, but
// this one leaked its existence. Registering it explicitly, gated by the
// same middleware, closes that gap.
//
// Exported so callers registering routes ahead of the dashboard (see
// server_routes.go's pre-registration collision check) can guard against
// this path too, not just DashboardRoute.
const SlashlessDashboardRoute = "/internal/ui"

// AccessMiddleware returns a handler that enforces the dashboard's
// network-layer policy before delegating to next. cfg must already be Validated
// (RegisterRoutes guarantees this); an unvalidated config with nil parsed CIDRs
// rejects every request, which is the safe failure mode. log may be nil: the
// registry has tests that construct handlers without one, and in that case
// rejections still surface in the response body, just without the Warn line.
func AccessMiddleware(cfg config.DashboardConfig, log logger.StyledLogger, next http.Handler) http.Handler {
	return accessMiddleware(cfg, log, next, newAccessWarnThrottle(log, nil, nil))
}

// newAccessWarnThrottle builds the denial Warn sampler. A blocked client can retry
// at wire speed, so the Warn is sampled per client IP; the 403 body is unaffected
// and every rejection still gets it. Nil when there is no logger to write to.
func newAccessWarnThrottle(log logger.StyledLogger, now func() time.Time, set *logger.ThrottleSet) *logger.Throttle {
	if log == nil {
		return nil
	}
	return set.Track(logger.NewRejectThrottle(log, "dashboard access denied", "dashboard", "client_ip", now))
}

// accessMiddleware is AccessMiddleware with the Warn throttle supplied by the
// caller, so both dashboard routes share one and tests can inject a clock.
func accessMiddleware(cfg config.DashboardConfig, log logger.StyledLogger, next http.Handler, warnLog *logger.Throttle) http.Handler {
	allowedHosts := make(map[string]struct{}, len(cfg.AccessPolicy.AllowedHosts))
	for _, h := range cfg.AccessPolicy.AllowedHosts {
		allowedHosts[strings.ToLower(normaliseHost(h))] = struct{}{}
	}
	parsed := cfg.ParsedCIDRs()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TCP source only (proxy trust off): the rate limiter honours X-Forwarded-For because the
		// operator opted into trusted proxy CIDRs there, but the dashboard has no such setting and
		// allowed_cidrs is an access control that a spoofable header must not be able to satisfy.
		clientIP := util.GetClientIP(r, false, nil)

		if !ipInAnyCIDR(clientIP, parsed) {
			reject(w, log, warnLog, "ip not in allowed range", "dashboard.access_policy.allowed_cidrs", clientIP, r.Host)
			return
		}

		host := strings.ToLower(normaliseHost(r.Host))
		if !hostAccepted(host, allowedHosts) {
			reject(w, log, warnLog, "host not accepted", "dashboard.access_policy.allowed_hosts", clientIP, r.Host)
			return
		}

		next.ServeHTTP(w, r)
	})
}

