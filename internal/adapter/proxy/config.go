package proxy

import (
	"time"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/core/ports"
)

const (
	// these are default values for proxy settings that should eventually be configurable
	// they're tuned for typical llm workloads but might need adjustment for specific use cases
	DefaultReadTimeout      = 60 * time.Second
	DefaultStreamBufferSize = 8 * 1024

	DefaultTimeout   = 60 * time.Second
	DefaultKeepAlive = 60 * time.Second

	// tcp connection tuning specifically optimised for ai streaming workloads
	// we need more connections than typical web apps because llm requests are long-lived
	// and we want to avoid connection exhaustion during traffic spikes
	OllaDefaultMaxIdleConns    = 100
	OllaDefaultMaxConnsPerHost = 50
	OllaDefaultIdleConnTimeout = 90 * time.Second
)

type Configuration struct {
	ProxyPrefix string
	Profile     string

	ConnectionTimeout     time.Duration
	ConnectionKeepAlive   time.Duration
	ResponseTimeout       time.Duration
	ReadTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	TLSHandshakeTimeout   time.Duration
	Retry                 proxyconfig.RetryPolicy
	StreamBufferSize      int

	// Olla-specific fields for advanced connection pooling
	IdleConnTimeout time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int
}
