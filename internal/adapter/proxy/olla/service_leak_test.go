package olla

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// pollCeiling and pollInterval bound the require.Eventually polls in this
// file: generous enough to absorb scheduler jitter, tight enough to keep the
// suite fast on the happy path.
const (
	pollCeiling  = 5 * time.Second
	pollInterval = 10 * time.Millisecond
)

// TestEndpointPoolCleanup_NoMemoryLeak verifies endpoint pools are cleaned up
func TestEndpointPoolCleanup_NoMemoryLeak(t *testing.T) {
	// Create a minimal service with cleanup enabled
	s := &Service{
		BaseProxyComponents: &core.BaseProxyComponents{
			Logger: createTestLogger(),
		},
		endpointPools:   *xsync.NewMap[string, *connectionPool](),
		circuitBreakers: *xsync.NewMap[string, *circuitBreaker](),
		cleanupTicker:   time.NewTicker(100 * time.Millisecond), // Fast cleanup for testing
		cleanupStop:     make(chan struct{}),
	}
	cfg := &Configuration{}
	cfg.MaxIdleConns = 10
	cfg.MaxConnsPerHost = 5
	cfg.IdleConnTimeout = 30 * time.Second
	s.configuration.Store(cfg)

	// Start cleanup loop
	go s.cleanupLoop()
	defer func() {
		close(s.cleanupStop)
		s.cleanupTicker.Stop()
	}()

	// Create many endpoint pools
	for i := range 50 {
		endpoint := fmt.Sprintf("endpoint-%d", i)
		pool := s.getOrCreateEndpointPool(endpoint)

		// Simulate some usage
		atomic.StoreInt64(&pool.lastUsed, time.Now().UnixNano())
	}

	// Verify pools were created
	poolCount := 0
	s.endpointPools.Range(func(k string, v *connectionPool) bool {
		poolCount++
		return true
	})
	if poolCount != 50 {
		t.Fatalf("Expected 50 pools, got %d", poolCount)
	}

	// Set all pools as old (last used > 5 minutes ago)
	oldTime := time.Now().Add(-10 * time.Minute).UnixNano()
	s.endpointPools.Range(func(k string, pool *connectionPool) bool {
		atomic.StoreInt64(&pool.lastUsed, oldTime)
		return true
	})

	// Wait for cleanup to run
	require.Eventually(t, func() bool {
		count := 0
		s.endpointPools.Range(func(k string, v *connectionPool) bool {
			count++
			return true
		})
		return count == 0
	}, pollCeiling, pollInterval, "pools were not cleaned up")
}

// TestCircuitBreakerCleanup_NoMemoryLeak verifies circuit breakers are cleaned up
func TestCircuitBreakerCleanup_NoMemoryLeak(t *testing.T) {
	s := &Service{
		BaseProxyComponents: &core.BaseProxyComponents{
			Logger: createTestLogger(),
		},
		endpointPools:   *xsync.NewMap[string, *connectionPool](),
		circuitBreakers: *xsync.NewMap[string, *circuitBreaker](),
		cleanupTicker:   time.NewTicker(100 * time.Millisecond),
		cleanupStop:     make(chan struct{}),
	}
	s.configuration.Store(&Configuration{})

	// Start cleanup loop
	go s.cleanupLoop()
	defer func() {
		close(s.cleanupStop)
		s.cleanupTicker.Stop()
	}()

	// Create circuit breakers without corresponding pools
	for i := range 30 {
		endpoint := fmt.Sprintf("endpoint-%d", i)
		cb := s.GetCircuitBreaker(endpoint)
		// Set as closed with no recent failures
		cb.SetOpen(false)
		cb.SetLastFailureNanos(0)
	}

	// Verify circuit breakers were created
	cbCount := 0
	s.circuitBreakers.Range(func(k string, v *circuitBreaker) bool {
		cbCount++
		return true
	})
	if cbCount != 30 {
		t.Fatalf("Expected 30 circuit breakers, got %d", cbCount)
	}

	// Wait for cleanup
	require.Eventually(t, func() bool {
		count := 0
		s.circuitBreakers.Range(func(k string, v *circuitBreaker) bool {
			count++
			return true
		})
		return count == 0
	}, pollCeiling, pollInterval, "circuit breakers were not cleaned up")
}

// TestCleanupLoop_ExitsCleanly verifies the cleanup goroutine exits properly
func TestCleanupLoop_ExitsCleanly(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	// Create and start multiple services
	for range 5 {
		s := &Service{
			BaseProxyComponents: &core.BaseProxyComponents{
				Logger: createTestLogger(),
			},
			endpointPools:   *xsync.NewMap[string, *connectionPool](),
			circuitBreakers: *xsync.NewMap[string, *circuitBreaker](),
			cleanupTicker:   time.NewTicker(50 * time.Millisecond),
			cleanupStop:     make(chan struct{}),
		}
		s.configuration.Store(&Configuration{})

		// Start cleanup loop
		go s.cleanupLoop()

		// Confirm the loop actually started (rather than blindly sleeping and
		// hoping) so it has genuinely been running before we stop it.
		require.Eventually(t, func() bool {
			return runtime.NumGoroutine() > initialGoroutines
		}, pollCeiling, pollInterval, "cleanup goroutine did not start")

		// Stop it
		close(s.cleanupStop)
		s.cleanupTicker.Stop()
	}

	// Wait for goroutines to exit
	require.Eventually(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine()-initialGoroutines <= 2 // allow small variance
	}, pollCeiling, pollInterval, "goroutine leak detected after cleanup loops stopped")
}
