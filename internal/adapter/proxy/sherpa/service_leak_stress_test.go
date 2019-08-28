package sherpa

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestPerformTimedRead_NoGoroutineLeak_Stress runs the original comprehensive leak test
// This test is skipped in CI (when -short flag is used)
func TestPerformTimedRead_NoGoroutineLeak_Stress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}
	// Get initial goroutine count
	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	s := &Service{
		configuration: func() *Configuration {
			c := &Configuration{}
			c.ReadTimeout = 100 * time.Millisecond // Original timeout
			return c
		}(),
	}

	state := &streamState{}
	logger := createTestLogger()

	// Run many reads that will timeout (original 50 iterations)
	for range 50 {
		ctx := context.Background()

