package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// BenchmarkCreateChainMiddleware_ServeHTTP measures the hot path: the chain is
// built once (as it is in production, at route-wiring time) and ServeHTTP is
// then invoked per request. Before the fix, CreateChainMiddleware's returned
// handler rebuilt the rate-limit and size-validation middleware on every
// single call to ServeHTTP; this benchmark is what catches that regression.
func BenchmarkCreateChainMiddleware_ServeHTTP(b *testing.B) {
	cfg := createTestConfig()
	cfg.Server.RateLimits.GlobalRequestsPerMinute = 1_000_000
	cfg.Server.RateLimits.PerIPRequestsPerMinute = 1_000_000
	cfg.Server.RateLimits.BurstSize = 1_000_000
