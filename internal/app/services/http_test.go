package services

import (
	"math"
	"net/http"
	"strings"
	"testing"
)

// TestMaxHeaderBytesFromConfig covers the derivation of http.Server's
// MaxHeaderBytes from server.request_limits.max_header_size: the configured
// value is used verbatim above the floor, zero/negative falls back to Go's
// own default rather than disabling the cap, and a small positive value is
// floored rather than crippling legitimate requests.
func TestMaxHeaderBytesFromConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		configure int64
