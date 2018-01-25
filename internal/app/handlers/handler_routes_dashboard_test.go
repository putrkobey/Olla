package handlers

// Route-mounting tests for the dashboard, scoped to the no-ripple design in
// simple-dashboard.md §5: registerRoutes() keeps its existing return type,
// the dashboard subtree mounts last, a collision logs Error and skips mounting
// rather than halting startup, and the access middleware stays scoped to
// /internal/ui/. The three GateInternalAPI subtests from
// feature/dashboard-impl are deliberately dropped: that wrapping is not
// implemented on this branch (§5.2).

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/router"
)

// enabledDashboardCfg returns a validated loopback-only DashboardConfig. This
// is the shipped default and the configuration under which any leak of the
// access middleware onto another route would be most visible (loopback-only
// rejects everything we probe with from outside 127.0.0.0/8).
func enabledDashboardCfg(t *testing.T) config.DashboardConfig {
	t.Helper()
	c := config.DashboardConfig{
		Enabled: true,
		AccessPolicy: config.AccessPolicyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8", "::1/128"},
			AllowedHosts: []string{"localhost", "127.0.0.1", "[::1]"},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("validate dashboard cfg: %v", err)
	}
	return c
}

// applicationWithStaticRouteTable builds an Application whose registerRoutes
// call produces the static provider route table (the superset production
// emits when YAML profile loading is unavailable). profileFactory and
// translatorRegistry are deliberately nil so registerProviderRoutes falls
// through to registerStaticProviderRoutes and registerTranslatorRoutes warns
// and returns; translator routes have their own dedicated coverage elsewhere.
func applicationWithStaticRouteTable(t *testing.T, dash config.DashboardConfig) (*Application, *router.RouteRegistry) {
	t.Helper()
	reg := router.NewRouteRegistry(&mockStyledLogger{})
	app := &Application{
		Config: &config.Config{
			Dashboard: dash,
		},
		logger:        &mockStyledLogger{},
		routeRegistry: reg,
	}
	app.registerRoutes()
	return app, reg
}

// TestFullRouteTable_DashboardMiddlewareStaysScoped is the full route table
// loop proving the dashboard access middleware stays scoped to /internal/ui/
// and does not silently attach to any other route. It enumerates every route
// the production registration path emits, replaces each non-dashboard handler
// with a per-route marker, then probes every route with a non-loopback
// RemoteAddr + non-allowed Host. Any non-dashboard route that returns the
// dashboard's signature 403 has had the access middleware leak onto it.
func TestFullRouteTable_DashboardMiddlewareStaysScoped(t *testing.T) {
	t.Parallel()

	_, reg := applicationWithStaticRouteTable(t, enabledDashboardCfg(t))

	// Replace every non-dashboard handler with a marker so leaks surface as a
	// body mismatch rather than a nil-handler panic. The dashboard subtree is
	// left untouched so the access middleware stays in place for it.
	mark := map[string]string{}
	for route := range reg.GetRoutes() {
		if strings.HasPrefix(route, dashboard.DashboardRoute) {
			continue
		}
		mark[route] = "MARK:" + route
		r := route
		reg.GetRoutes()[r] = router.RouteInfo{
			Handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("MARK:" + r))
			},
			Description: "marker",
			Method:      http.MethodGet,
			Order:       reg.GetRoutes()[r].Order,
		}
	}

	mux := http.NewServeMux()
	reg.WireUp(mux)

	// Every non-dashboard route must reach its own marker unchanged when
	// probed with a RemoteAddr/Host the dashboard middleware would reject.
	// A 403 here means the access gate leaked onto that route.
	for route, want := range mark {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		req.RemoteAddr = "203.0.113.9:54321"
		req.Host = "evil.example"
