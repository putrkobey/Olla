package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/proxy"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	appconfig "github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// noopDiscoveryService and noopEndpointSelector are the minimal stand-ins
// needed to construct a proxy.Factory service - these tests only assert on
// the effective configuration a constructed engine ends up with, never
// route a request.
type noopDiscoveryService struct{}

func (noopDiscoveryService) GetEndpoints(context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}
func (noopDiscoveryService) GetHealthyEndpoints(context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}
func (noopDiscoveryService) RefreshEndpoints(context.Context) error { return nil }
func (noopDiscoveryService) UpdateEndpointStatus(context.Context, *domain.Endpoint) error {
	return nil
}

type noopEndpointSelector struct{}

func (noopEndpointSelector) Select(context.Context, []*domain.Endpoint) (*domain.Endpoint, error) {
	return nil, nil
}
func (noopEndpointSelector) Name() string                          { return "noop" }
func (noopEndpointSelector) IncrementConnections(*domain.Endpoint) {}
func (noopEndpointSelector) DecrementConnections(*domain.Endpoint) {}

// TestProxyConfiguration_RealChain_OllaGetsEngineBufferDefault is the
// regression test for the real F1 bug: it wasn't enough for the factory to
// copy the raw (possibly zero) StreamBufferSize - config.DefaultConfig()
// hardcoded it to 8KiB, and both shipped config.yaml files also set it to
// 8192 explicitly. Either alone permanently defeats Olla's 64KiB zero-check
// no matter how correct the factory's copy logic is. This goes through the
// actual production chain - config.DefaultConfig / config.Load ->
// ProxyServiceWrapper.createProxyConfiguration -> proxy.Factory - not a bare
// struct, which is the only way a regression at either of those layers would
// be caught.
func TestProxyConfiguration_RealChain_OllaGetsEngineBufferDefault(t *testing.T) {
	logger := newTestLogger()
	collector := stats.NewCollector(logger)
	discovery := noopDiscoveryService{}
	selector := noopEndpointSelector{}

	buildOllaService := func(t *testing.T, cfg *appconfig.Config) *olla.Service {
