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

func (c *HTTPModelDiscoveryClient) DiscoverModels(ctx context.Context, endpoint *domain.Endpoint) (models []*domain.ModelInfo, err error) {
	defer func() {
		if err != nil {
			c.recordError(endpoint.URLString)
		}
	}()
	startTime := time.Now()

	c.updateMetrics(func(m *DiscoveryMetrics) {
		atomic.AddInt64(&m.TotalDiscoveries, 1)
	})

	profileType := endpoint.Type
	if profileType == "" || profileType == domain.ProfileAuto {
		return c.discoverWithAutoDetection(ctx, endpoint, startTime)
	}

	platformProfile, err := c.profileFactory.GetProfile(profileType)
	if err != nil {
		return nil, NewDiscoveryError(endpoint.URLString, profileType, "get_profile", 0, time.Since(startTime), err)
	}

	return c.discoverWithProfile(ctx, endpoint, platformProfile, startTime)
}

// discoverWithAutoDetection tries profiles in order until one succeeds
func (c *HTTPModelDiscoveryClient) discoverWithAutoDetection(ctx context.Context, endpoint *domain.Endpoint, startTime time.Time) ([]*domain.ModelInfo, error) {
	// We're going to try profiles in order:
	//	Ollama → llama.cpp → LM Studio → vLLM → OpenAI Compatible (as a last resort)
	// Resolution for this may change in the future as more front-ends appear or are added.
	profileTypes := []string{
		domain.ProfileOllama,
		domain.ProfileLlamaCpp,
		domain.ProfileLmStudio,
		domain.ProfileVLLM,
		domain.ProfileOMLX,             // Apple Silicon only; tried after other common servers
		domain.ProfileOpenAICompatible, /* last ditch effort */
	}

	var lastErr error
	for _, profileType := range profileTypes {
		platformProfile, err := c.profileFactory.GetProfile(profileType)
		if err != nil {
			lastErr = err
			continue
		}

		models, err := c.discoverWithProfile(ctx, endpoint, platformProfile, startTime)
		if err == nil {
			c.logger.InfoWithEndpoint(" Setting up", endpoint.Name, "profile", platformProfile.GetName())
			return models, nil
		}

		if ctx.Err() != nil {
			return nil, err
		}

		lastErr = err
	}

	return nil, NewDiscoveryError(endpoint.URLString, domain.ProfileAuto, "auto_detect", 0, time.Since(startTime), lastErr)
}

// discoverWithProfile performs discovery using a specific platform profile
func (c *HTTPModelDiscoveryClient) discoverWithProfile(ctx context.Context, endpoint *domain.Endpoint, platformProfile domain.PlatformProfile, startTime time.Time) ([]*domain.ModelInfo, error) {
	// For specific profiles like OpenAI-compatible, skip model_url override by default
	// to avoid breaking existing configurations that rely on profile defaults
	// Issue #86 fix applies mainly to LlamaCpp, Ollama, etc. where users need custom paths
	var discoveryURL string

	// Skip model_url override for OpenAI-compatible unless it's a custom path
