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
	if _, exists := routes[dashboard.SlashlessDashboardRoute]; exists {
		a.logger.Error("dashboard route collision; skipping mount",
			"route", dashboard.SlashlessDashboardRoute)
		return
	}
	if !a.Config.Dashboard.Enabled {
		return
	}
	dashboard.RegisterRoutes(a.routeRegistry, a.Config.Dashboard, a.logger, dashboardHandlerFactory(), a.throttles)
}

// translatorRejectWriter returns the writer that formats a security rejection in the
// translator's own error schema, or nil (plain text) when it has none. The quiet variant
// is preferred: the rate limiter already reports the rejection through its sampled log.
func translatorRejectWriter(trans translator.RequestTranslator) security.RejectFunc {
	var write func(w http.ResponseWriter, err error, statusCode int)
	switch ew := trans.(type) {
	case translator.QuietErrorWriter:
		write = ew.WriteErrorQuiet
	case translator.ErrorWriter:
		write = ew.WriteError
	default:
		return nil
	}
	return func(w http.ResponseWriter, status int, message string) {
		// The access-log middleware that assigns the request ID sits behind the security
		// chain, so a rejection has none yet; SDKs read request-id for correlation.
		if w.Header().Get(constants.HeaderXOllaRequestID) == "" {
			w.Header().Set(constants.HeaderXOllaRequestID, util.GenerateRequestID())
		}
		write(w, errors.New(message), status)
	}
}

// registerTranslatorRoutes dynamically registers routes for all translators
// Translators that implement PathProvider interface provide their own API paths
// This enables adding new translators without modifying the routing code
func (a *Application) registerTranslatorRoutes() {
	if a.translatorRegistry == nil {
		a.logger.Warn("Translator registry not available, skipping translator routes")
		return
	}

	translators := a.translatorRegistry.GetAll()
	a.logger.InfoWithCount("Starting registering translator routes", len(translators))

	for name, trans := range translators {
		// Check if translator implements PathProvider for dynamic route registration
		// Translators without PathProvider must be registered manually
		if pathProvider, ok := trans.(translator.PathProvider); ok {
			path := pathProvider.GetAPIPath()
			handler := a.translationHandler(trans)
			reject := translatorRejectWriter(trans)
			bodyLimit := translatorBodyLimit(trans)

			// Translator routes are inference entry points (Claude Code's main path), so
			// they register through RegisterTranslatorRoute to sit behind the rate and
			// size limits like proxy routes. A translator added later is covered too.
			a.routeRegistry.RegisterTranslatorRoute(
				path,
				handler,
				name+" Messages API",
				"POST",
				reject,
				bodyLimit,
			)

			// Register models endpoint for translators
			// Uses base path from translator + "/models" (e.g., /olla/anthropic/v1/models)
			modelsPath := extractBasePath(path) + "/models"
			modelsHandler := a.translatorModelsHandler(trans)

			a.routeRegistry.RegisterTranslatorRoute(
				modelsPath,
				modelsHandler,
				name+" Models API",
				"GET",
				reject,
				bodyLimit,
			)

			// Register token counting endpoint for translators that support it
			// Uses message path + "_count_tokens" (e.g., /olla/anthropic/v1/messages/count_tokens)
			if _, ok := trans.(translator.TokenCounter); ok {
				tokenCountPath := path + "/count_tokens"
				tokenCountHandler := a.tokenCountHandler(trans)

				a.routeRegistry.RegisterTranslatorRoute(
					tokenCountPath,
					tokenCountHandler,
					name+" Token Count API",
					"POST",
					reject,
					bodyLimit,
				)

				a.logger.Debug("Registered translator routes",
					"translator", name,
					"messages_path", path,
					"models_path", modelsPath,
					"token_count_path", tokenCountPath)

				a.logger.InfoWithEndpoint(" ", name, "path", path)
			} else {
