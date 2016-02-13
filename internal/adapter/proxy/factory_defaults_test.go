package proxy

import (
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
)

// TestFactory_UnsetConfig_AppliesEngineDefaults is the regression test for the
// zero-check-defeated bug: the factory used to copy already-defaulted getter
// values (generic 8KiB / 60s) into the engine config before the engine's own
// NewService zero-checks ran, so Olla's 64KiB buffer default and its own
// read-timeout default never fired. Constructing through the real factory
// path with a bare, unset Configuration is the only way to catch this - unit
// tests against the engines directly bypass the factory copy entirely.
func TestFactory_UnsetConfig_AppliesEngineDefaults(t *testing.T) {
	logger := createTestLogger()
	collector := createTestStatsCollector()
	factory := NewFactory(collector, nil, logger)
	discovery := &mockDiscoveryService{}
	selector := newMockEndpointSelector(collector)

	t.Run("olla", func(t *testing.T) {
		svc, err := factory.Create(DefaultProxyOlla, discovery, selector, &Configuration{})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		ollaSvc, ok := svc.(*olla.Service)
		if !ok {
			t.Fatalf("expected *olla.Service, got %T", svc)
