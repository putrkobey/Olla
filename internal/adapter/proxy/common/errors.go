package common

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrNoHealthyEndpoints is returned when no healthy endpoints are available
	ErrNoHealthyEndpoints = errors.New("no healthy endpoints available")
)

// MakeUserFriendlyError converts technical errors into user-friendly messages with actionable guidance
// all implementations should use this function to ensure consistent error handling with detailed context
// for TUI output and logging.
//
//nolint:gocognit // intentionally complex; we could break it down further, but this is already quite readable
func MakeUserFriendlyError(err error, duration time.Duration, errorContext string, responseTimeout time.Duration) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, context.Canceled):
		// Common client timeout pattern (curl default, browser timeouts, etc.)
		if duration >= 25*time.Second && duration <= 35*time.Second {
			return fmt.Errorf("request cancelled after %.1fs - likely client timeout (increase client timeout to 60-90s for LLM responses)", duration.Seconds())
		}
		// Very quick cancellations suggest network issues
		if duration < 2*time.Second {
			return fmt.Errorf("request cancelled after %.1fs - client disconnected immediately (check network connectivity)", duration.Seconds())
		}
