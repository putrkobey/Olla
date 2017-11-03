package sherpa

import (
	"context"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/logger"
)

// pollCeiling and pollInterval bound the require.Eventually goroutine-count
// polls in this file: generous enough to absorb scheduler jitter, tight
// enough to keep the suite fast on the happy path.
const (
	pollCeiling  = 5 * time.Second
	pollInterval = 10 * time.Millisecond
)

// TestCreateCombinedContext_NoGoroutineLeak verifies that the context merger doesn't leak goroutines
func TestCreateCombinedContext_NoGoroutineLeak(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	s := &Service{}

	// Create many contexts and ensure goroutines are cleaned up
	for i := range 100 {
		ctx1, cancel1 := context.WithCancel(context.Background())
		ctx2, cancel2 := context.WithCancel(context.Background())

		combinedCtx, combinedCancel := s.createCombinedContext(ctx1, ctx2)

		// Cancel one of the contexts
		if i%2 == 0 {
			cancel1()
		} else {
			cancel2()
		}

		// Wait for the combined context to be cancelled
		<-combinedCtx.Done()

		// Clean up
		combinedCancel()
		cancel1()
		cancel2()
	}

	// Wait for goroutines to exit
	require.Eventually(t, func() bool {
		runtime.GC()
		return runtime.NumGoroutine()-initialGoroutines <= 2 // allow small variance
	}, pollCeiling, pollInterval, "goroutine leak detected")
}

// TestPerformTimedRead_NoGoroutineLeak verifies read operations don't leak goroutines
func TestPerformTimedRead_NoGoroutineLeak(t *testing.T) {
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	s := &Service{
		configuration: func() *Configuration {
			c := &Configuration{}
			c.ReadTimeout = 10 * time.Millisecond // Much shorter for tests
			return c
		}(),
	}

	logger := createTestLogger()

	// Run timeout tests concurrently to speed up
	var wg sync.WaitGroup
	const numTimeoutTests = 20 // Reduced from 50

	for range numTimeoutTests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()

			// Create a new state for each goroutine to avoid race conditions
			localState := &streamState{}

			// Use a reader that delays then returns data
			reader := &delayedReader{delay: 50 * time.Millisecond} // Still longer than timeout

			// Create a new buffer for each goroutine
			localBuffer := make([]byte, 1024)

			// This should timeout after 10ms
			result, err := s.performTimedRead(ctx, reader, localBuffer, 10*time.Millisecond, localState, logger)
			if err == nil && result != nil {
				t.Error("Expected timeout but got result")
			}
		}()
	}

	// Test context cancellation concurrently too
	const numCancelTests = 20 // Reduced from 50
	for range numCancelTests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())

			// Create a new state for each goroutine to avoid race conditions
			localState := &streamState{}

			// Use a reader that returns data immediately
			reader := strings.NewReader("test data")

			// Create a new buffer for each goroutine
			localBuffer := make([]byte, 1024)

			// Cancel context immediately
			cancel()

			// This should return nil due to context cancellation
