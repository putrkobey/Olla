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
