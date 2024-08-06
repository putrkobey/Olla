package handlers

// Regression coverage for mountDashboard's enabled gate: dashboard.Handler()
// (walking and SHA-256-hashing the whole embedded SPA bundle to build its
// per-asset cache) must only run when the dashboard is actually enabled. It
// used to be evaluated unconditionally as a call argument before
// dashboard.RegisterRoutes checked cfg.Enabled internally, so a disabled
// dashboard paid that cost for nothing.

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/router"
)

// TestMountDashboard_Disabled_DoesNotConstructHandler substitutes
// dashboardHandlerFactory with a spy to prove it is never invoked when the
// dashboard is disabled, closing the gap a purely route-table-based
// assertion could not: the old code produced the same empty route table
// whether or not it had wastefully built the handler first.
func TestMountDashboard_Disabled_DoesNotConstructHandler(t *testing.T) {
	orig := dashboardHandlerFactory
	t.Cleanup(func() { dashboardHandlerFactory = orig })
