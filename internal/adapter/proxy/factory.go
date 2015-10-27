package proxy

import (
	"fmt"
	"sync"
	"time"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	DefaultProxySherpa = "sherpa"
	DefaultProxyOlla   = "olla"
)

// ProxyCreator is a function that creates a proxy service
type ProxyCreator func(ports.DiscoveryService, domain.EndpointSelector, ports.ProxyConfiguration, ports.StatsCollector, ports.MetricsExtractor, logger.StyledLogger) (ports.ProxyService, error)

// Factory creates proxy services using a registry pattern
type Factory struct {
	creators         map[string]ProxyCreator
	statsCollector   ports.StatsCollector
	metricsExtractor ports.MetricsExtractor
	logger           logger.StyledLogger
	mu               sync.RWMutex
}

func NewFactory(statsCollector ports.StatsCollector, metricsExtractor ports.MetricsExtractor, theLogger logger.StyledLogger) *Factory {
	factory := &Factory{
		creators:         make(map[string]ProxyCreator),
		statsCollector:   statsCollector,
		metricsExtractor: metricsExtractor,
		logger:           theLogger,
	}

	// Register Sherpa implementation
	factory.Register(DefaultProxySherpa, func(discovery ports.DiscoveryService, selector domain.EndpointSelector, config ports.ProxyConfiguration, collector ports.StatsCollector, metricsExtractor ports.MetricsExtractor, logger logger.StyledLogger) (ports.ProxyService, error) {
		sherpaConfig := &sherpa.Configuration{}
		sherpaConfig.ProxyPrefix = config.GetProxyPrefix()
		sherpaConfig.ConnectionTimeout = config.GetConnectionTimeout()
		sherpaConfig.ConnectionKeepAlive = config.GetConnectionKeepAlive()
		sherpaConfig.ResponseTimeout = config.GetResponseTimeout()
		sherpaConfig.ReadTimeout = rawReadTimeout(config)
		sherpaConfig.StreamBufferSize = rawStreamBufferSize(config)
		sherpaConfig.Profile = config.GetProxyProfile()
		// Each optional getter is asserted independently so a config that supports
		// only one timeout does not silently drop the other back to its default.
		if rh, ok := config.(interface{ GetResponseHeaderTimeout() time.Duration }); ok {
			sherpaConfig.ResponseHeaderTimeout = rh.GetResponseHeaderTimeout()
		}
		if tls, ok := config.(interface{ GetTLSHandshakeTimeout() time.Duration }); ok {
			sherpaConfig.TLSHandshakeTimeout = tls.GetTLSHandshakeTimeout()
		}
		if rp, ok := config.(interface {
			GetRetryPolicy() proxyconfig.RetryPolicy
		}); ok {
			sherpaConfig.Retry = rp.GetRetryPolicy()
		}
		return sherpa.NewService(discovery, selector, sherpaConfig, collector, metricsExtractor, logger)
	})

	// Register Olla implementation
	factory.Register(DefaultProxyOlla, func(discovery ports.DiscoveryService, selector domain.EndpointSelector, config ports.ProxyConfiguration, collector ports.StatsCollector, metricsExtractor ports.MetricsExtractor, logger logger.StyledLogger) (ports.ProxyService, error) {
		ollaConfig := &olla.Configuration{}
		ollaConfig.ProxyPrefix = config.GetProxyPrefix()
		ollaConfig.ConnectionTimeout = config.GetConnectionTimeout()
		ollaConfig.ConnectionKeepAlive = config.GetConnectionKeepAlive()
		ollaConfig.ResponseTimeout = config.GetResponseTimeout()
		ollaConfig.ReadTimeout = rawReadTimeout(config)
		ollaConfig.StreamBufferSize = rawStreamBufferSize(config)
		ollaConfig.Profile = config.GetProxyProfile()

		// Pool tunables travel together and define the "is this an Olla-specific
		// config" check, so they stay in one assertion.
		if ollaSpecific, ok := config.(interface {
			GetMaxIdleConns() int
			GetIdleConnTimeout() time.Duration
			GetMaxConnsPerHost() int
