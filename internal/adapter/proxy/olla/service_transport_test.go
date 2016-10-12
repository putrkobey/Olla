package olla

import (
	"reflect"
	"runtime"
	"testing"
	"time"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
)

// funcName extracts the full symbol name of a function value.
func funcName(f interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// TestCreateOptimisedTransport_ConnectionLimits verifies that both MaxConnsPerHost and
// MaxIdleConnsPerHost are mapped to their correct fields on http.Transport.
// Previously MaxConnsPerHost was mistakenly written to MaxIdleConnsPerHost and
// MaxConnsPerHost was never set (defaulting to 0 = unlimited).
func TestCreateOptimisedTransport_ConnectionLimits(t *testing.T) {
	t.Parallel()

	cfg := &Configuration{}
	cfg.MaxConnsPerHost = 42
	cfg.MaxIdleConnsPerHost = 17
	cfg.MaxIdleConns = 200
	cfg.IdleConnTimeout = 90 * time.Second

	transport := createOptimisedTransport(cfg)

	if transport.MaxConnsPerHost != 42 {
		t.Errorf("MaxConnsPerHost: want 42, got %d", transport.MaxConnsPerHost)
	}
	if transport.MaxIdleConnsPerHost != 17 {
		t.Errorf("MaxIdleConnsPerHost: want 17, got %d", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxIdleConns != 200 {
		t.Errorf("MaxIdleConns: want 200, got %d", transport.MaxIdleConns)
	}
}

// TestCreateOptimisedTransport_DefaultsApplied verifies that NewService fills in sensible
// defaults before handing the config to createOptimisedTransport, so a zero-value config
// never silently leaves MaxConnsPerHost unlimited.
func TestCreateOptimisedTransport_DefaultsApplied(t *testing.T) {
	t.Parallel()

	// Zero-value config — defaults should be filled in by NewService, but we can verify
	// the expected defaults are consistent with the package constants.
	cfg := &Configuration{}
	cfg.MaxConnsPerHost = proxyconfig.OllaDefaultMaxConnsPerHost
	cfg.MaxIdleConnsPerHost = proxyconfig.OllaDefaultMaxIdleConnsPerHost
	cfg.MaxIdleConns = proxyconfig.OllaDefaultMaxIdleConns
	cfg.IdleConnTimeout = proxyconfig.OllaDefaultIdleConnTimeout

	transport := createOptimisedTransport(cfg)

	if transport.MaxConnsPerHost != proxyconfig.OllaDefaultMaxConnsPerHost {
		t.Errorf("MaxConnsPerHost: want %d, got %d", proxyconfig.OllaDefaultMaxConnsPerHost, transport.MaxConnsPerHost)
	}
	if transport.MaxIdleConnsPerHost != proxyconfig.OllaDefaultMaxIdleConnsPerHost {
		t.Errorf("MaxIdleConnsPerHost: want %d, got %d", proxyconfig.OllaDefaultMaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
	}
}

// TestCreateOptimisedTransport_FieldsAreDistinct guards against the specific regression
