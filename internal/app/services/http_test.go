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
		want      int
	}{
		{"configured value used verbatim", 2 * 1024 * 1024, 2 * 1024 * 1024},
		{"default config value (1MiB)", 1024 * 1024, 1024 * 1024},
		{"zero falls back to Go's default, not disabled", 0, http.DefaultMaxHeaderBytes},
		{"negative falls back to Go's default, not disabled", -1, http.DefaultMaxHeaderBytes},
		{"small positive value floored", 100, minMaxHeaderBytes},
		{"exactly the floor stays unchanged", minMaxHeaderBytes, minMaxHeaderBytes},
		{"huge value clamped to MaxInt on overflow", math.MaxInt64, math.MaxInt},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := maxHeaderBytesFromConfig(tc.configure)
			if got != tc.want {
				t.Errorf("maxHeaderBytesFromConfig(%d) = %d, want %d", tc.configure, got, tc.want)
			}
		})
	}
}
