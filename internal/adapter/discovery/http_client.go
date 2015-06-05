package discovery

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/version"

	"github.com/putrkobey/Olla/internal/adapter/factory"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	DefaultTimeout     = 30 * time.Second
	MaxResponseSize    = 10 * 1024 * 1024 // 10MB limit for model responses
	DefaultUserAgent   = "%s-Discovery/%s"
	DefaultContentType = constants.DefaultContentTypeJSON

	DefaultMaxIdleConnections        = 10
	DefaultIdleConnTimeout           = 60 * time.Second
	DefaultMaxIdleConnectionsPerHost = 5
)

// HTTPModelDiscoveryClient implements ModelDiscoveryClient using HTTP requests
type HTTPModelDiscoveryClient struct {
	httpClient     *http.Client
	profileFactory *profile.Factory
	logger         logger.StyledLogger
	metrics        DiscoveryMetrics
	mu             sync.RWMutex
}

// NewHTTPModelDiscoveryClient uses httpClient verbatim. Discovery sends the
// endpoint's auth and custom headers, so production callers must build the
// client with factory.NewBackendHTTPClient (no environment proxy, no
// cross-origin redirects).
func NewHTTPModelDiscoveryClient(profileFactory *profile.Factory, logger logger.StyledLogger, httpClient *http.Client) *HTTPModelDiscoveryClient {
	return &HTTPModelDiscoveryClient{
		httpClient:     httpClient,
		profileFactory: profileFactory,
		logger:         logger,
		metrics: DiscoveryMetrics{
			ErrorsByEndpoint: make(map[string]int64),
		},
	}
}

func NewHTTPModelDiscoveryClientWithDefaults(profileFactory *profile.Factory, logger logger.StyledLogger) *HTTPModelDiscoveryClient {
	return &HTTPModelDiscoveryClient{
		httpClient: factory.NewBackendHTTPClient(factory.BackendClientOptions{
			Timeout:             DefaultTimeout,
			MaxIdleConns:        DefaultMaxIdleConnections,
			IdleConnTimeout:     DefaultIdleConnTimeout,
			MaxIdleConnsPerHost: DefaultMaxIdleConnectionsPerHost,
		}),
		profileFactory: profileFactory,
		logger:         logger,
		metrics: DiscoveryMetrics{
			ErrorsByEndpoint: make(map[string]int64),
		},
	}
}
