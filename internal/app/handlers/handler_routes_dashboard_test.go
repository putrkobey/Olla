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
