package sherpa

import (
	"reflect"
	"runtime"
	"testing"
	"time"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
)

// funcName extracts the full symbol name of a function value for comparison.
// http.ProxyFromEnvironment is a named function so the pointer is stable across builds.
func funcName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// newSherpaServiceForTransportTest builds a real Sherpa service via NewService so
// the transport tests exercise the production construction path.
func newSherpaServiceForTransportTest(t *testing.T) *Service {
	t.Helper()

	cfg := &Configuration{}
	cfg.ConnectionTimeout = 2 * time.Second
	cfg.ConnectionKeepAlive = 30 * time.Second
	cfg.StreamBufferSize = 8192

	svc, err := NewService(
		nil, // discovery service, not needed for transport tests
		&mockEndpointSelector{},
		cfg,
		nil, // stats collector
		nil, // metrics extractor
		createTestLogger(),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Cleanup)
	return svc
}
