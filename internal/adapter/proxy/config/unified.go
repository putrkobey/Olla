package config

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// Default values for proxy settings
const (
	DefaultReadTimeout      = 60 * time.Second
	DefaultStreamBufferSize = 8 * 1024
	DefaultTimeout          = 60 * time.Second
	DefaultKeepAlive        = 60 * time.Second

	// Olla-specific defaults for high-performance
	OllaDefaultStreamBufferSize    = 64 * 1024 // Larger buffer for better streaming performance
	OllaDefaultMaxIdleConns        = 100
	OllaDefaultMaxConnsPerHost     = 50
	OllaDefaultMaxIdleConnsPerHost = 25 // Half of MaxConnsPerHost; idle slots rarely need to match total capacity
	OllaDefaultIdleConnTimeout     = 90 * time.Second
	// Olla uses 30s connection/keep-alive timeouts for faster failure detection in AI
	// workloads, but ReadTimeout stays at 60s to match DefaultReadTimeout - streaming
	// inference responses can legitimately go quiet between tokens for longer than a
	// connection-establishment timeout would tolerate.
	OllaDefaultTimeout     = 30 * time.Second
	OllaDefaultKeepAlive   = 30 * time.Second
	OllaDefaultReadTimeout = 60 * time.Second

	// DefaultResponseHeaderTimeout caps the time a backend may hold the connection
	// open after accepting without sending a single response header byte.
	// 30 s is chosen to match Olla's other timeout defaults; Sherpa uses the same constant.
	DefaultResponseHeaderTimeout = 30 * time.Second

	// DefaultHealthResponseHeaderTimeout is shorter than the proxy timeout because
	// health probes are latency-sensitive and already bounded by CheckTimeout.
	DefaultHealthResponseHeaderTimeout = 10 * time.Second

	// DefaultTLSHandshakeTimeout caps the TLS negotiation phase. 10 s matches the
	// Go stdlib default and is sufficient for local inference backends; slow TLS
	// handshakes on these hosts usually indicate a misconfiguration.
	DefaultTLSHandshakeTimeout = 10 * time.Second
)

// RetryPolicy controls failover between endpoints for a single request. Fields
// are phrased so the zero value is today's behaviour (fail over to every
// eligible endpoint once, replaying any method after a connection failure);
// a Configuration built without a policy therefore never silently loses failover.
type RetryPolicy struct {
	// MaxAttempts caps attempts per request; 0 means every eligible endpoint once.
	MaxAttempts int
	// Disabled makes every request a single attempt with no failover.
	Disabled bool
	// NoNonIdempotent stops POST, PATCH and DELETE failing over after a connection
	// failure. GET, HEAD and OPTIONS still do.
	NoNonIdempotent bool
}

// ProxyConfig defines the interface for all proxy configurations
type ProxyConfig interface {
	// Core configuration getters
	GetProxyProfile() string
	GetProxyPrefix() string
	GetConnectionTimeout() time.Duration
	GetConnectionKeepAlive() time.Duration
	GetResponseTimeout() time.Duration
	GetReadTimeout() time.Duration
	GetStreamBufferSize() int

	// Validation
	Validate() error
}

// BaseProxyConfig contains common configuration fields for all proxy implementations
type BaseProxyConfig struct {
	ProxyPrefix           string
	Profile               string
	ConnectionTimeout     time.Duration
	ConnectionKeepAlive   time.Duration
	ResponseTimeout       time.Duration
	ReadTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	Retry                 RetryPolicy
	StreamBufferSize      int
}

// GetRetryPolicy returns the failover policy applied to each proxied request.
func (c *BaseProxyConfig) GetRetryPolicy() RetryPolicy {
	return c.Retry
}

// GetResponseHeaderTimeout returns the maximum time the transport waits for a
// backend's first response header, defaulting to DefaultResponseHeaderTimeout.
// Raise it for backends that load models on demand (e.g. Lemonade), where the
// first request blocks until the model is resident and the 30s default would
// abort a legitimate cold start.
func (c *BaseProxyConfig) GetResponseHeaderTimeout() time.Duration {
	if c.ResponseHeaderTimeout == 0 {
		return DefaultResponseHeaderTimeout
	}
	return c.ResponseHeaderTimeout
}

// GetTLSHandshakeTimeout returns the TLS handshake timeout, defaulting to
// DefaultTLSHandshakeTimeout. Exposed so operators can extend it for backends
// behind slow TLS terminators, though the default covers all local backends.
func (c *BaseProxyConfig) GetTLSHandshakeTimeout() time.Duration {
	if c.TLSHandshakeTimeout == 0 {
		return DefaultTLSHandshakeTimeout
	}
	return c.TLSHandshakeTimeout
}

// GetProxyProfile returns the proxy profile, defaulting to "auto" if not set
func (c *BaseProxyConfig) GetProxyProfile() string {
	if c.Profile == "" {
		return constants.ConfigurationProxyProfileAuto
	}
	return c.Profile
}

// GetProxyPrefix returns the proxy prefix, defaulting to "/olla/" if not set
func (c *BaseProxyConfig) GetProxyPrefix() string {
	if c.ProxyPrefix == "" {
