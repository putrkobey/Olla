package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// Config holds all configuration for the application
type Config struct {

	// ModelAliases maps a virtual model name (alias) to a list of actual model names
	// that backends may know. When a request arrives with an alias model name, olla
	// resolves it to all configured actual model names, finds endpoints that serve any
	// of them, and rewrites the model name in the request to match the selected backend.
	// Example:
	//   model_aliases:
	//     gpt-oss-120b:
	//       - gpt-oss:120b
	//       - gpt-oss-120b-MLX
	//       - gguf_gpt_oss_120b.gguf
	ModelAliases  map[string][]string `yaml:"model_aliases,omitempty"`
	Logging       LoggingConfig       `yaml:"logging"`
	Filename      string              `yaml:"-"`
	UnknownKeys   []UnknownKey        `yaml:"-"` // keys in the file that nothing reads
	Dashboard     DashboardConfig     `yaml:"dashboard"`
	Translators   TranslatorsConfig   `yaml:"translators"`
	ModelRegistry ModelRegistryConfig `yaml:"model_registry"`
	Discovery     DiscoveryConfig     `yaml:"discovery"`
	Server        ServerConfig        `yaml:"server"`
	Proxy         ProxyConfig         `yaml:"proxy"`
	Engineering   EngineeringConfig   `yaml:"engineering"`
}

// CorsConfig controls browser cross-origin access to the Olla API.
// This matters when a browser-based client (OpenWebUI, a custom dashboard, etc.)
// calls Olla from a different origin. Non-browser clients (curl, SDKs, other
// services) ignore CORS headers entirely, so enabling this has no effect on them.
//
// Security note: combining AllowCredentials=true with AllowedOrigins=["*"] is
// forbidden by the CORS spec — the browser will reject the response. Validate()
// enforces this at startup rather than letting it silently break at runtime.
//
// ExposedHeaders is intentionally left empty here; the middleware phase will
// automatically expose the full X-Olla-* response header set.
type CorsConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	MaxAge           int      `yaml:"max_age"`
	Enabled          bool     `yaml:"enabled"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host              string              `yaml:"host"`
	Cors              CorsConfig          `yaml:"cors"`
	RateLimits        ServerRateLimits    `yaml:"rate_limits"`
	RequestLimits     ServerRequestLimits `yaml:"request_limits"`
	Port              int                 `yaml:"port"`
	ReadTimeout       time.Duration       `yaml:"read_timeout"`
	ReadHeaderTimeout time.Duration       `yaml:"read_header_timeout"`
	WriteTimeout      time.Duration       `yaml:"write_timeout"`
	IdleTimeout       time.Duration       `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration       `yaml:"shutdown_timeout"`
	RequestLogging    bool                `yaml:"request_logging"`
}

// Validate checks CORS configuration for spec-violating combinations that would
// silently break browser clients at runtime rather than failing loudly at startup.
func (c *CorsConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	// rs/cors treats an empty AllowedOrigins list as allow-all, which is the
	// opposite of what an operator expects when they write allowed_origins: [].
	// Require an explicit ["*"] to opt into allow-all so the intent is unambiguous.
	if len(c.AllowedOrigins) == 0 {
		return errors.New("cors: allowed_origins must not be empty when cors is enabled; set [\"*\"] to allow all origins explicitly")
	}
	if c.MaxAge < 0 {
		return fmt.Errorf("cors.max_age must be non-negative, got %d", c.MaxAge)
	}
	// The CORS spec forbids Access-Control-Allow-Origin: * when credentials are in play.
	// Browsers will refuse the response, so this is a startup error rather than a warning.
	if c.AllowCredentials {
		for _, origin := range c.AllowedOrigins {
			if origin == "*" {
				return errors.New("cors: allow_credentials=true is incompatible with allowed_origins=[\"*\"]; list explicit origins instead")
			}
		}
	}
	return nil
}

// AccessPolicyConfig is a network-layer allow rule shared by the dashboard's
// gated zones. A request must pass both checks; there is no auth behind them,
// so network restriction is the only control.
type AccessPolicyConfig struct {
	// AllowedCIDRs is matched against the TCP-layer source from r.RemoteAddr
	// only. Proxy headers (X-Forwarded-For, X-Real-IP) are never consulted,
	// under any configuration: Olla's listener legitimately binds 0.0.0.0 and
	// trusting those headers without a separate trust mechanism would let any
	// client spoof past the gate. Required, non-empty, when the zone using
	// this policy is active.
	AllowedCIDRs []string `yaml:"allowed_cidrs"`

	// AllowedHosts is matched against r.Host with the :port suffix stripped
	// and the comparison case-insensitive. May be empty: any Host that parses
	// as an IP literal is always accepted, so an operator browsing
	// via http://192.168.1.10:40114/internal/ui/ needs no entry here. List
	// only the non-IP hostnames you want to allow (e.g. a LAN mDNS name).
	AllowedHosts []string `yaml:"allowed_hosts"`

	// allowedCIDRsParsed is populated by Validate so the hot request path
	// never re-parses. Validate must run once at startup before any handler
	// wrapped by this policy accepts a request.
	allowedCIDRsParsed []*net.IPNet
}

// DashboardConfig controls the embedded read-only admin dashboard, served from
// the existing HTTP listener at /internal/ui/.
type DashboardConfig struct {
	// AccessPolicy gates /internal/ui/* unconditionally whenever Enabled is
	// true. GateInternalAPI exists for forward compatibility but is inert on
	// this branch (see GateInternalAPI).
	AccessPolicy AccessPolicyConfig `yaml:"access_policy"`

	// Enabled controls whether /internal/ui/* routes are registered at all.
	// The embedded assets are always compiled into the binary; this only
	// governs whether they are ever served.
	Enabled bool `yaml:"enabled"`

	// GateInternalAPI is reserved for a later change that opts the rest of
	// /internal/* (and /version) into the same AccessPolicy. It is INERT for
	// now: setting it true has no effect. Defaults to false because
	// defaulting it on would silently break existing deployments that scrape
	// /internal/metrics or poll /internal/health from hosts the policy would
	// reject. The field ships now so that later change needs no config migration.
	GateInternalAPI bool `yaml:"gate_internal_api"`
