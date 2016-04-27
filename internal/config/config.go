package config

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/util"

	"github.com/docker/go-units"
	"gopkg.in/yaml.v3"
)

const (
	DefaultPort              = 40114
	DefaultHost              = "localhost"
	DefaultAllHost           = "0.0.0.0" // local dev may use this
	DefaultProxyProfile      = constants.ConfigurationProxyProfileAuto
	DefaultProxyEngine       = "olla"
	DefaultLoadBalancer      = "least-connections"
	DefaultModelRegistryType = "memory"
	DefaultDiscoveryType     = "static"
)

// ptrInt returns a pointer to the given int value.
// Used for config fields where nil means "not set by the user".
func ptrInt(v int) *int { return &v }

var DefaultLocalNetworkTrustedCIDRs = []string{
	"127.0.0.0/8",
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:            DefaultHost,
			Port:            DefaultPort,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    0, // No timeout for proxies
			ShutdownTimeout: 10 * time.Second,
			RequestLogging:  true,
			RequestLimits: ServerRequestLimits{
				MaxBodySize:   100 * 1024 * 1024,
				MaxHeaderSize: 1024 * 1024,
			},
			RateLimits: ServerRateLimits{
				GlobalRequestsPerMinute: 1000,
				PerIPRequestsPerMinute:  100,
				BurstSize:               50,
				HealthRequestsPerMinute: 1000,
				CleanupInterval:         5 * time.Minute,
				TrustProxyHeaders:       false,
				TrustedProxyCIDRs:       DefaultLocalNetworkTrustedCIDRs,
				TrustedProxyCIDRsParsed: nil, // Will be parsed later
			},
			// CORS is off by default; operators opt-in when browser clients need access.
			// ExposedHeaders is empty here — the middleware phase populates the X-Olla-* set.
			Cors: CorsConfig{
				Enabled:          false,
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
				AllowedHeaders:   []string{"*"},
				ExposedHeaders:   nil,
				AllowCredentials: false,
				MaxAge:           300,
			},
		},
		Proxy: ProxyConfig{
			Engine:       DefaultProxyEngine,
			LoadBalancer: DefaultLoadBalancer,
			Profile:      DefaultProxyProfile,
			// StreamBufferSize is deliberately left at zero here: each proxy
			// engine defaults it differently (Olla 64KiB, Sherpa 8KiB) via its
			// own zero-check further down the chain. Hardcoding a value here
			// would defeat that - see factory.go's raw (zero-preserving) copy.
			ConnectionTimeout: 60 * time.Second,
			ResponseTimeout:   15 * time.Minute,
			ReadTimeout:       10 * time.Minute,
			// Zero, not the old 3 and 500ms: both are deprecated and unused, and a
			// non-zero default would make "operator set it" undetectable for the
			// startup warning.
			Retry: RetryConfig{Enabled: true, NonIdempotent: true},
			StickySessions: StickySessionConfig{
				Enabled:         false,
				IdleTTLSeconds:  600,
				MaxSessions:     10000,
				KeySources:      []string{"session_header", "prefix_hash", "auth_header"},
				PrefixHashBytes: 512,
			},
		},
		Discovery: DiscoveryConfig{
			Type:            DefaultDiscoveryType,
			RefreshInterval: 30 * time.Second,
			Static: StaticDiscoveryConfig{
				Endpoints: []EndpointConfig{
					{
						URL:            "http://localhost:11434",
						Name:           "localhost",
						Type:           "ollama",
						Priority:       ptrInt(100),
						HealthCheckURL: "/health",
						ModelURL:       "/api/tags",
						CheckInterval:  5 * time.Second,
						CheckTimeout:   2 * time.Second,
					},
				},
			},
			ModelDiscovery: ModelDiscoveryConfig{
				Enabled:           true,
				Interval:          5 * time.Minute,
				Timeout:           30 * time.Second,
				ConcurrentWorkers: 5,
				RetryAttempts:     3,
				RetryBackoff:      1 * time.Second,
			},
		},
		ModelRegistry: ModelRegistryConfig{
			Type:          DefaultModelRegistryType,
			EnableUnifier: true,
			RoutingStrategy: ModelRoutingStrategy{
				Type: "strict", // default to strict for predictable behavior
				Options: ModelRoutingStrategyOptions{
					DiscoveryRefreshOnMiss: false,
					DiscoveryTimeout:       2 * time.Second,
					FallbackBehavior:       constants.FallbackBehaviorCompatibleOnly,
				},
			},
			Unification: UnificationConfig{
				// Mirror config.yaml so a no-config-file run prunes on the same cadence
				// as the shipped config. These are honoured by the unifier (via
				// CreateWithConfig), so the values genuinely take effect.
				StaleThreshold:  24 * time.Hour,
				CleanupInterval: 10 * time.Minute,
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
		Engineering: EngineeringConfig{
			ShowNerdStats: false,
		},
		Dashboard: DashboardConfig{
			Enabled: true,
			AccessPolicy: AccessPolicyConfig{
				AllowedCIDRs: []string{"127.0.0.0/8", "::1/128"},
				// "localhost" is a hostname, not an IP literal, so the
				// auto-accept for IP-literal Hosts does not cover it. DefaultHost
				// is itself "localhost", so a no-config-file startup (go install,
				// curl installer) serving on 0.0.0.0 must not 403 its own default.
				AllowedHosts: []string{"localhost"},
			},
			// Inert for now: the GateInternalAPI wrapping is not implemented yet.
			// The field ships now so a later change needs no config migration.
			GateInternalAPI: false,
		},
		Translators: TranslatorsConfig{
			Anthropic: AnthropicTranslatorConfig{
				Enabled:            true,
				PassthroughEnabled: true,
				MaxMessageSize:     10 << 20, // 10MB - Anthropic API limit,
				Inspector: InspectorConfig{
					Enabled:       false,
					OutputDir:     "logs/inspector/anthropic",
					SessionHeader: "X-Session-ID",
				},
			},
		},
	}
}

// Validate checks for dangerous zero/empty values that would cause panics or
// silent failures at runtime. It is called after all config sources (file, env
// overrides) have been applied, so the final merged state is what's checked.
func (c *Config) Validate() error {
	if c.Discovery.Type == "" {
		return errors.New("discovery.type must not be empty (e.g. \"static\")")
	}
	if c.Proxy.Engine == "" {
		return errors.New("proxy.engine must not be empty (e.g. \"sherpa\" or \"olla\")")
	}
	if c.Proxy.LoadBalancer == "" {
		return errors.New("proxy.load_balancer must not be empty (e.g. \"priority\")")
	}
	if c.Proxy.Retry.MaxAttempts < 0 {
		return fmt.Errorf("proxy.retry.max_attempts must be >= 0 (0 = every eligible endpoint once), got %d", c.Proxy.Retry.MaxAttempts)
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}

	if c.Discovery.ModelDiscovery.Enabled {
		md := c.Discovery.ModelDiscovery
		if md.Interval <= 0 {
			return errors.New("discovery.model_discovery.interval must be > 0 when model discovery is enabled (prevents ticker panic)")
		}
		if md.ConcurrentWorkers <= 0 {
			return errors.New("discovery.model_discovery.concurrent_workers must be > 0 when model discovery is enabled (prevents errgroup panic)")
		}
		if md.Timeout <= 0 {
			return errors.New("discovery.model_discovery.timeout must be > 0 when model discovery is enabled (prevents immediate context expiry)")
		}
		// Zero is valid (retry immediately); a negative value would only ever be a typo.
		if md.RetryBackoff < 0 {
			return errors.New("discovery.model_discovery.retry_backoff must be >= 0")
		}
	}

	// A burst size of zero throttles all traffic immediately — warn rather than
	// error since zero is technically valid in some rate-limiter implementations.
	if c.Server.RateLimits.BurstSize == 0 {
		log.Printf("WARNING: server.rate_limits.burst_size is 0 — all requests will be throttled immediately")
	}

	if err := c.Translators.Validate(); err != nil {
		return fmt.Errorf("translators config invalid: %w", err)
	}

	if err := c.Server.Cors.Validate(); err != nil {
		return fmt.Errorf("server.cors config invalid: %w", err)
	}

	if err := c.Dashboard.Validate(); err != nil {
		return err
	}

	return nil
}

func Load(flagConfigFile ...string) (*Config, error) {
	config := DefaultConfig()

	// Simple config file loading - check a few standard locations
	configPaths := []string{"config/config.local.yaml", "config/config.yaml", "config.yaml", "default.yaml"}

	// Priority: flag > environment variable > default paths
	if len(flagConfigFile) > 0 && flagConfigFile[0] != "" {
		configPaths = []string{flagConfigFile[0]}
	} else if configFile := os.Getenv("OLLA_CONFIG_FILE"); configFile != "" {
		configPaths = []string{configFile}
	}

	var configLoaded bool
	var configFilename string

	for _, path := range configPaths {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, config); err != nil {
				return nil, fmt.Errorf("failed to parse %s: %w", path, err)
			}
			configLoaded = true
			configFilename = path
			// Lax decoding is kept so old configs still load; the unknown keys
			// are surfaced to the operator instead of failing the load.
			config.UnknownKeys = FindUnknownKeys(data)
