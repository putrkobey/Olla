package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/metrics"
	"github.com/putrkobey/Olla/internal/adapter/proxy"
	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	// defaultConnectionKeepAlive is the fallback when the proxy config omits
	// connection_keep_alive. Matches the app-layer default to keep behaviour
	// identical for operators who haven't set the field.
	defaultConnectionKeepAlive = 30 * time.Second
)

// ProxyServiceWrapper adapts the core proxy implementation to the service lifecycle
// model. It manages the creation of load balancers and proxy engines, ensuring they
// receive validated endpoints from the discovery service.
type ProxyServiceWrapper struct {
	config       *config.ProxyConfig
	proxyService ports.ProxyService
	loadBalancer domain.EndpointSelector
	// stickyWrapper is non-nil when sticky sessions are enabled; held separately
	// so Stop() can shut down its background goroutine.
	stickyWrapper    *balancer.StickySessionWrapper
	endpointRepo     domain.EndpointRepository
	discoveryService ports.DiscoveryService
	statsCollector   ports.StatsCollector
	metricsExtractor ports.MetricsExtractor
	statsService     *StatsService
	discoverySvc     *DiscoveryService
	securityService  *SecurityService
	logger           logger.StyledLogger
}

// NewProxyServiceWrapper creates a new proxy service wrapper
func NewProxyServiceWrapper(
	config *config.ProxyConfig,
	logger logger.StyledLogger,
) *ProxyServiceWrapper {
	return &ProxyServiceWrapper{
		config: config,
		logger: logger,
	}
}

// Name returns the service name
func (s *ProxyServiceWrapper) Name() string {
	return "proxy"
}

// Start initialises the proxy service and load balancer
