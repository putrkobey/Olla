package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
	"github.com/putrkobey/Olla/theme"
)

// plainTranslator hides everything but the base translator interface and a path, standing
// in for a translator added later that has no custom error format of its own.
type plainTranslator struct {
	translator.RequestTranslator
}

func (plainTranslator) GetAPIPath() string { return "/olla/plain/v1/messages" }

// translatorSecurity builds adapters that allow `burst` rapid requests per client before
// limiting, and cap bodies at maxBody bytes. The global limit is off so that only the per-IP
// bucket can answer 429: the tests then prove per-IP limiting of these routes, not the global
// bucket draining. The collector is returned so tests can read the recorded violations.
func translatorSecurity(t *testing.T, burst int, maxBody int64) (*security.Adapters, *stats.Collector) {
	t.Helper()

	cfg := &config.Config{Server: config.ServerConfig{
		RateLimits: config.ServerRateLimits{
			GlobalRequestsPerMinute: 0,
			PerIPRequestsPerMinute:  60,
			BurstSize:               burst,
		},
		RequestLimits: config.ServerRequestLimits{MaxBodySize: maxBody},
	}}
	quiet := logger.NewStyledLogger(slog.New(slog.NewTextHandler(io.Discard, nil)), &theme.Theme{}, false)
	collector := stats.NewCollector(quiet)
	_, adapters := security.NewSecurityServices(cfg, collector, &mockStyledLogger{})
	t.Cleanup(adapters.Stop)
	return adapters, collector
}

// wireTranslatorRoutes registers the translator routes of a real engine-backed app and
// wires them through the production security chain.
func wireTranslatorRoutes(t *testing.T, engine string, adapters *security.Adapters) (*router.RouteRegistry, http.Handler) {
	t.Helper()

	ep := liveEndpoint(t, "ollama-host", "ollama", newPathBackend(t, rootChatPath).URL, 50)
	f := newTranslationPathApp(t, engine, false, false, []*domain.Endpoint{ep}, map[string][]*domain.Endpoint{"m": {ep}})

	reg := router.NewRouteRegistry(&mockStyledLogger{})
	f.app.routeRegistry = reg
	f.app.translatorRegistry = translator.NewRegistry(&mockStyledLogger{})
	f.app.translatorRegistry.Register("anthropic", f.trans)
	f.app.translatorRegistry.Register("plain", plainTranslator{f.trans})
