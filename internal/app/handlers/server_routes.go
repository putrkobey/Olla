package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
)

// staticRoute captures the minimal routing info needed when profile loading is unavailable
type staticRoute struct {
	path        string
	handler     http.HandlerFunc
	description string
	method      string
	isProxy     bool
	rateLimited bool // takes the proxy security chain without being the catch-all
}

// staticProvider bundles routes with their URL prefixes for test scenarios
type staticProvider struct {
	prefixes []string
	routes   []staticRoute
}

// dashboardHandlerFactory constructs the dashboard's static-asset handler. A
// package-level var (rather than calling dashboard.Handler directly) so tests
// can substitute a cheap stub and observe whether construction happened,
// without adding test-only instrumentation to the dashboard package itself.
var dashboardHandlerFactory = dashboard.Handler

// registerRoutes sets up the complete HTTP routing table
func (a *Application) registerRoutes() {
	// Internal health and monitoring endpoints come first - they're critical
	// for operations and shouldn't depend on any provider configuration
	a.routeRegistry.RegisterWithMethod(constants.DefaultHealthCheckEndpoint, a.healthHandler, "Health check endpoint", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/status", middleware.GzipFunc(a.statusHandler), "Endpoint status", "GET")
	a.routeRegistry.RegisterWithMethod(constants.DefaultMetricsEndpoint, a.metricsHandler, "Prometheus metrics", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/status/endpoints", middleware.GzipFunc(a.endpointsStatusHandler), "Endpoints status", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/status/models", middleware.GzipFunc(a.modelsStatusHandler), "Models status", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/stats/models", a.modelStatsHandler, "Model statistics", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/stats/translators", a.translatorStatsHandler, "Translator statistics", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/stats/sticky", a.stickyStatsHandler, "Sticky session statistics", "GET")
	a.routeRegistry.RegisterWithMethod("/internal/process", a.processStatsHandler, "Process status", "GET")
	a.routeRegistry.RegisterWithMethod("/version", a.versionHandler, "Olla version information", "GET")

	// Unified model views aggregate across all providers
	a.routeRegistry.RegisterWithMethod("/olla/models", a.unifiedModelsHandler, "Unified models listing with filtering", "GET")
	a.routeRegistry.RegisterWithMethod(constants.PathOllaModelsPrefix, a.unifiedModelByAliasHandler, "Get unified model by ID or alias", "GET")

	// Legacy proxy paths for backward compatibility
	a.routeRegistry.RegisterProxyRoute("/olla/proxy/", a.proxyHandler, "Olla API proxy endpoint (sherpa)", "POST")
	a.routeRegistry.RegisterWithMethod("/olla/proxy/v1/models", a.openaiModelsHandler, "OpenAI-compatible models", "GET")

	// Dynamic translator route registration
	// Each translator that implements PathProvider gets its route automatically registered
	// This scales to unlimited translators (Gemini, Bedrock, etc.) without code changes
	a.registerTranslatorRoutes()

	// Provider routes are built from YAML configs when available
	a.registerProviderRoutes()

	// Dashboard mounts last so its /internal/ui/ subtree cannot shadow any
	// provider or internal route registered above.
	a.mountDashboard()
}

// mountDashboard registers the dashboard subtree, gated by cfg.Enabled. The
// collision check is belt-and-braces defence against a future regression:
// every route registered above is either an /olla/ prefixed proxy route or a
// fixed literal distinct from /internal/ui/ and /internal/ui, so this is
// never expected to fire. On detecting a collision we log Error and skip
// mounting (fail safe: never mount something shadowed, never take the server
// down over a defensive check). See simple-dashboard.md §5.
//
// Both routes RegisterRoutes claims - the trailing-slash subtree and the
// exact slashless redirect - are checked here. registerWithMethod stores
// routes in a plain map keyed by pattern, so a pre-existing registration on
// either path would otherwise be silently overwritten (last write wins, no
// panic) rather than caught.
//
// dashboard.Handler() is only called from inside the enabled branch below,
// not passed as an argument evaluated unconditionally: constructing it walks
// and SHA-256-hashes the entire embedded SPA bundle (~300KB) to build the
// per-asset cache, real work that a disabled dashboard has no reason to pay.
// Keep the enabled check textually gating the Handler() call so this stays
// obviously true by inspection rather than relying on evaluation-order
// subtlety.
func (a *Application) mountDashboard() {
	routes := a.routeRegistry.GetRoutes()
	if _, exists := routes[dashboard.DashboardRoute]; exists {
		a.logger.Error("dashboard route collision; skipping mount",
			"route", dashboard.DashboardRoute)
		return
	}
