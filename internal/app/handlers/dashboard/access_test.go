package dashboard

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
)

// stubLogger satisfies logger.StyledLogger for the RouteRegistry without
// pulling in the real themed logger. The registry only uses it for the route
// table print, which these tests do not assert on.
type stubLogger struct{}

func (stubLogger) Debug(msg string, args ...any)                                {}
func (stubLogger) Info(msg string, args ...any)                                 {}
func (stubLogger) Warn(msg string, args ...any)                                 {}
func (stubLogger) Error(msg string, args ...any)                                {}
func (stubLogger) ResetLine()                                                   {}
func (stubLogger) InfoWithStatus(msg string, status string, args ...any)        {}
func (stubLogger) InfoWithCount(msg string, count int, args ...any)             {}
func (stubLogger) InfoWithEndpoint(msg string, endpoint string, args ...any)    {}
func (stubLogger) InfoWithHealthCheck(msg string, endpoint string, args ...any) {}
func (stubLogger) InfoWithNumbers(msg string, numbers ...int64)                 {}
func (stubLogger) WarnWithEndpoint(msg string, endpoint string, args ...any)    {}
func (stubLogger) ErrorWithEndpoint(msg string, endpoint string, args ...any)   {}
func (stubLogger) InfoHealthy(msg string, endpoint string, args ...any)         {}
func (stubLogger) InfoHealthStatus(msg string, name string, status domain.EndpointStatus, args ...any) {
}
func (stubLogger) GetUnderlying() *slog.Logger                                         { return slog.Default() }
func (stubLogger) WithRequestID(requestID string) logger.StyledLogger                  { return nil }
func (stubLogger) InfoConfigChange(oldName, newName string)                            {}
func (stubLogger) WithAttrs(attrs ...slog.Attr) logger.StyledLogger                    { return nil }
func (stubLogger) With(args ...any) logger.StyledLogger                                { return nil }
func (stubLogger) InfoWithContext(msg string, endpoint string, ctx logger.LogContext)  {}
func (stubLogger) WarnWithContext(msg string, endpoint string, ctx logger.LogContext)  {}
func (stubLogger) ErrorWithContext(msg string, endpoint string, ctx logger.LogContext) {}

func loopbackConfig() config.DashboardConfig {
	c := config.DashboardConfig{
		Enabled: true,
		AccessPolicy: config.AccessPolicyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8", "::1/128"},
			AllowedHosts: []string{"localhost", "127.0.0.1", "[::1]"},
		},
	}
	if err := c.Validate(); err != nil {
		panic(err)
	}
	return c
}

// reachedHandler flips a flag and returns 200 when the wrapped handler runs.
func reachedHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func doRequest(t *testing.T, h http.Handler, remoteAddr, host, forwardedFor, realIP string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, DashboardRoute, nil)
	req.RemoteAddr = remoteAddr
	req.Host = host
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAccessMiddleware_RejectionMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		remoteAddr string
		host       string
		xff        string
		realIP     string
		wantStatus int
		// wantContains is asserted on the 403 body when wantStatus is 403:
		// the self-diagnosing response must echo both the rejected IP and Host.
		wantContains string
	}{
		{"allowed CIDR + allowed host", "127.0.0.1:54321", "localhost", "", "", http.StatusOK, ""},
		{"disallowed IP + allowed host", "203.0.113.9:54321", "localhost", "", "", http.StatusForbidden, "203.0.113.9"},
		{"allowed IP + disallowed non-IP host", "127.0.0.1:54321", "evil.example", "", "", http.StatusForbidden, "evil.example"},
		{"both disallowed", "203.0.113.9:54321", "evil.example", "", "", http.StatusForbidden, "203.0.113.9"},
		// Critical: spoofed proxy headers must NOT bypass a non-loopback RemoteAddr.
		// These are the test cases that would have caught the DNS-rebinding-adjacent
		// class of bug where a middleware accidentally trusts XFF/X-Real-IP.
		{"spoofed XFF from non-loopback still 403", "203.0.113.9:54321", "localhost", "127.0.0.1", "", http.StatusForbidden, "203.0.113.9"},
		{"spoofed X-Real-IP from non-loopback still 403", "203.0.113.9:54321", "localhost", "", "127.0.0.1", http.StatusForbidden, "203.0.113.9"},
		{"spoofed both headers from non-loopback still 403", "203.0.113.9:54321", "localhost", "127.0.0.1", "127.0.0.1", http.StatusForbidden, "203.0.113.9"},
		// Common spoofing patterns that bypass naive XFF parsers: multi-hop
		// lists, leading/trailing whitespace, tab separators. The middleware
		// never consults the header at all, so all must reject identically.
		{"multi-hop XFF with allowed first", "203.0.113.9:54321", "localhost", "127.0.0.1, 203.0.113.9", "", http.StatusForbidden, "203.0.113.9"},
		{"multi-hop XFF with allowed last", "203.0.113.9:54321", "localhost", "203.0.113.9, 127.0.0.1", "", http.StatusForbidden, "203.0.113.9"},
		{"XFF with leading whitespace", "203.0.113.9:54321", "localhost", "  127.0.0.1", "", http.StatusForbidden, "203.0.113.9"},
		{"XFF with embedded tab", "203.0.113.9:54321", "localhost", "127.0.0.1\t,10.0.0.1", "", http.StatusForbidden, "203.0.113.9"},
		{"XFF via Forwarded RFC 7239 style", "203.0.113.9:54321", "localhost", "for=127.0.0.1", "", http.StatusForbidden, "203.0.113.9"},
		{"IPv6 XFF spoof", "203.0.113.9:54321", "localhost", "::1", "", http.StatusForbidden, "203.0.113.9"},
		// Any Host that parses as an IP literal is accepted regardless of
		// allowed_hosts. DNS rebinding requires a hostname resolution, so an
		// IP-literal Host is by definition the address the browser dialled and
		// cannot be the vehicle for a rebinding attack. The loopbackConfig()
		// used here does list 127.0.0.1/[::1] in allowed_hosts, but the cases
		// below with non-allowlisted IPs prove the IP-literal path itself works.
		{"IPv6 loopback with bracketed host", "[::1]:54321", "[::1]", "", "", http.StatusOK, ""},
		{"Host with port suffix matches bare entry", "127.0.0.1:54321", "127.0.0.1:41141", "", "", http.StatusOK, ""},
		{"Host case-insensitive", "127.0.0.1:54321", "LOCALHOST", "", "", http.StatusOK, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reached := false
			h := AccessMiddleware(loopbackConfig(), nil, reachedHandler(&reached))

			rec := doRequest(t, h, tc.remoteAddr, tc.host, tc.xff, tc.realIP)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.wantStatus, rec.Code, rec.Body.String())
			}
			switch tc.wantStatus {
			case http.StatusOK:
				if !reached {
					t.Fatal("handler must be reached on 200")
				}
			case http.StatusForbidden:
				if reached {
					t.Fatal("handler must NOT be reached on 403")
				}
				body := rec.Body.String()
				if !strings.Contains(body, tc.wantContains) {
					t.Fatalf("403 body must contain %q (self-diagnosing), got %q", tc.wantContains, body)
				}
				// Self-diagnosing body must echo BOTH rejected dimensions so the
				// operator can see what to fix without re-reading YAML.
				if !strings.Contains(body, "host=") || !strings.Contains(body, "ip=") {
					t.Fatalf("403 body must be self-diagnosing (ip=, host=), got %q", body)
				}
				// Must also name the config key to adjust, not just echo the
				// rejected values - "self-diagnosing" means telling the operator
				// where to fix it, not only what was rejected.
				if !strings.Contains(body, "dashboard.access_policy.allowed_hosts") || !strings.Contains(body, "allowed_cidrs") {
					t.Fatalf("403 body must name the config key to adjust, got %q", body)
				}
				if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
					t.Fatalf("403 Content-Type must be text/plain, got %q", ct)
				}
			}
		})
	}
}

// TestAccessMiddleware_IPLiteralHostAccepted is the dedicated proof that
// with an empty allowed_hosts, an IP-literal Host still passes because DNS
// rebinding cannot ride an IP address. This matches a Docker setup where
// allowed_hosts is empty and the operator browses by IP.
func TestAccessMiddleware_IPLiteralHostAccepted(t *testing.T) {
	t.Parallel()

	cfg := config.DashboardConfig{
		Enabled: true,
		AccessPolicy: config.AccessPolicyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8", "::1/128", "192.168.1.0/24"},
			// allowed_hosts deliberately empty: IP-literal Hosts must still pass.
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	cases := []struct {
		name       string
		remoteAddr string
		host       string
		want       int
	}{
		{"ipv4 loopback literal", "127.0.0.1:54321", "127.0.0.1", http.StatusOK},
		{"ipv4 loopback literal with port", "127.0.0.1:54321", "127.0.0.1:41141", http.StatusOK},
		{"ipv6 loopback bracketed", "[::1]:54321", "[::1]", http.StatusOK},
		{"ipv6 loopback bracketed with port", "[::1]:54321", "[::1]:41141", http.StatusOK},
		{"lan ipv4 literal", "192.168.1.10:54321", "192.168.1.10", http.StatusOK},
		{"non-IP host rejected when hosts empty", "127.0.0.1:54321", "localhost", http.StatusForbidden},
		{"non-IP host rejected when hosts empty (lan)", "192.168.1.10:54321", "olla.internal", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reached := false
			h := AccessMiddleware(cfg, nil, reachedHandler(&reached))
			rec := doRequest(t, h, tc.remoteAddr, tc.host, "", "")
			if rec.Code != tc.want {
				t.Fatalf("status: want %d, got %d (body=%q)", tc.want, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAccessMiddleware_RejectsEmptyRemoteAddr(t *testing.T) {
	t.Parallel()

	reached := false
	h := AccessMiddleware(loopbackConfig(), nil, reachedHandler(&reached))

	// Empty RemoteAddr: net.ParseIP("") returns nil, must fall through to 403
	// rather than panicking. Production code must not panic on weird inputs.
	rec := doRequest(t, h, "", "localhost", "", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("empty RemoteAddr must 403, got %d", rec.Code)
	}
	if reached {
		t.Fatal("handler must not be reached")
	}
}

func TestAccessMiddleware_NilParsedCIDRsDeniesAll(t *testing.T) {
	t.Parallel()

	// Defence in depth: if someone hands us a config that skipped Validate
	// (nil parsed CIDRs), every request must be rejected rather than silently
	// allowed. The registration helper always Validates first, so this only
	// fires on a programming error.
	reached := false
	h := AccessMiddleware(config.DashboardConfig{Enabled: true}, nil, reachedHandler(&reached))

	rec := doRequest(t, h, "127.0.0.1:54321", "localhost", "", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("nil parsed CIDRs must deny-all, got %d", rec.Code)
	}
}

func TestRegisterRoutes_DisabledLeavesMuxEmpty(t *testing.T) {
	t.Parallel()

	reg := router.NewRouteRegistry(stubLogger{})
	registered := RegisterRoutes(reg, config.DashboardConfig{Enabled: false}, nil, http.NotFoundHandler(), nil)
	if registered {
		t.Fatal("RegisterRoutes must report false when disabled")
	}
	if len(reg.GetRoutes()) != 0 {
		t.Fatalf("no routes should be registered when disabled, got %v", reg.GetRoutes())
	}
}

func TestRegisterRoutes_EnabledMountsDashboardSubtree(t *testing.T) {
	t.Parallel()

	reg := router.NewRouteRegistry(stubLogger{})
	registered := RegisterRoutes(reg, loopbackConfig(), nil, reachedHandler(boolPtr(t)), nil)
	if !registered {
		t.Fatal("RegisterRoutes must report true when enabled")
	}
	routes := reg.GetRoutes()
	info, ok := routes[DashboardRoute]
	if !ok {
		t.Fatalf("expected route %q registered, got %v", DashboardRoute, routes)
	}
	if info.Method != http.MethodGet {
		t.Fatalf("dashboard must be GET-only, got %q", info.Method)
	}
}

// TestSlashlessDashboardRoute_DisallowedClientGets403 is the regression test
// for the redirect-bypasses-policy gap: without an explicit registration for
// the exact "/internal/ui" path, Go's ServeMux answers it with its own
// redirect before AccessMiddleware ever runs, so a disallowed client got a
// 307 (revealing the mount exists) instead of the self-diagnosing 403 every
// other dashboard path returns. A disallowed client hitting the slashless
// path now gets the same 403 as any other dashboard path.
func TestSlashlessDashboardRoute_DisallowedClientGets403(t *testing.T) {
	t.Parallel()

	reg := router.NewRouteRegistry(stubLogger{})
	if !RegisterRoutes(reg, loopbackConfig(), nil, http.NotFoundHandler(), nil) {
		t.Fatal("RegisterRoutes must report true when enabled")
	}

	mux := http.NewServeMux()
	reg.WireUp(mux)

	req := httptest.NewRequest(http.MethodGet, SlashlessDashboardRoute, nil)
	req.RemoteAddr = "203.0.113.9:54321"
	req.Host = "evil.example"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("disallowed client on %s: status=%d, want 403 (got a redirect instead of the policy check)", SlashlessDashboardRoute, rec.Code)
	}
}

// TestSlashlessDashboardRoute_AllowedClientRedirects confirms the positive
// path: a client the access policy approves still gets the expected redirect
// to the canonical trailing-slash route, with the same security headers as
// every other dashboard response.
func TestSlashlessDashboardRoute_AllowedClientRedirects(t *testing.T) {
	t.Parallel()

	reg := router.NewRouteRegistry(stubLogger{})
	if !RegisterRoutes(reg, loopbackConfig(), nil, http.NotFoundHandler(), nil) {
		t.Fatal("RegisterRoutes must report true when enabled")
	}

	mux := http.NewServeMux()
	reg.WireUp(mux)

	req := httptest.NewRequest(http.MethodGet, SlashlessDashboardRoute, nil)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Host = "localhost"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("allowed client on %s: status=%d, want 307", SlashlessDashboardRoute, rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != DashboardRoute {
		t.Errorf("redirect Location=%q, want %q", loc, DashboardRoute)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("slashless redirect must carry the standard dashboard security headers")
	}
}

// TestRegisterRoutes_DoesNotTouchOtherRoutes is the regression test for a
// prior prototype's documented defect: that branch modified the
// registry's wiring logic and removed size-limit enforcement from every
// non-proxy route as a side effect of mounting the dashboard. This test wires
// a representative set of existing non-proxy routes alongside the dashboard and
// asserts each one still behaves byte-identically when the dashboard is
// enabled: same status, body still produced by the original handler, and no
// dashboard access check applied to them (a non-loopback RemoteAddr that the
// dashboard would 403 still reaches the underlying handler).
func TestRegisterRoutes_DoesNotTouchOtherRoutes(t *testing.T) {
	t.Parallel()

	reg := router.NewRouteRegistry(stubLogger{})

	mark := map[string]string{}
	add := func(route, marker string) {
		mark[route] = marker
		reg.RegisterWithMethod(route, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(marker))
		}), marker, http.MethodGet)
	}
