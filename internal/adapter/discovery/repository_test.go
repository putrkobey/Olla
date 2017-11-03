package discovery

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
)

// ptrInt is a test helper to construct *int literals inline.
func ptrInt(v int) *int { return &v }

// TestApplyEndpointDefaults_SetsZeroValues verifies that zero-value timing and priority
// fields are filled with the package defaults, ensuring a minimal YAML config can
// pass validation without requiring explicit timing fields.
func TestApplyEndpointDefaults_SetsZeroValues(t *testing.T) {
	t.Parallel()

	cfg := &config.EndpointConfig{
		Name: "test",
		URL:  "http://localhost:11434",
		// CheckInterval, CheckTimeout and Priority are intentionally omitted (nil/zero)
	}

	applyEndpointDefaults(cfg)

	if cfg.CheckInterval != DefaultCheckInterval {
		t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, DefaultCheckInterval)
	}
	if cfg.CheckTimeout != DefaultCheckTimeout {
		t.Errorf("CheckTimeout = %v, want %v", cfg.CheckTimeout, DefaultCheckTimeout)
	}
	if cfg.Priority == nil || *cfg.Priority != DefaultPriority {
		t.Errorf("Priority = %v, want %d", cfg.Priority, DefaultPriority)
	}
}

// TestApplyEndpointDefaults_PreservesExplicitValues confirms that explicitly configured
// values are never overwritten by the defaults — the function must only fill gaps.
func TestApplyEndpointDefaults_PreservesExplicitValues(t *testing.T) {
	t.Parallel()

	cfg := &config.EndpointConfig{
		Name:          "test",
		URL:           "http://localhost:11434",
		CheckInterval: 10 * time.Second,
		CheckTimeout:  3 * time.Second,
		Priority:      ptrInt(50),
	}

	applyEndpointDefaults(cfg)

	if cfg.CheckInterval != 10*time.Second {
		t.Errorf("CheckInterval = %v, want %v (should not be overwritten)", cfg.CheckInterval, 10*time.Second)
	}
	if cfg.CheckTimeout != 3*time.Second {
		t.Errorf("CheckTimeout = %v, want %v (should not be overwritten)", cfg.CheckTimeout, 3*time.Second)
	}
	if cfg.Priority == nil || *cfg.Priority != 50 {
		t.Errorf("Priority = %v, want 50 (should not be overwritten)", cfg.Priority)
	}
}

// TestApplyEndpointDefaults_PreservesExplicitZeroPriority is a regression test for the
// silent override bug: priority: 0 in YAML was indistinguishable from "omitted" when the
// field was a plain int, causing the default (100) to silently replace a user-set 0.
// With Priority as *int, nil means "omitted" and a pointer-to-zero means "explicitly 0".
func TestApplyEndpointDefaults_PreservesExplicitZeroPriority(t *testing.T) {
	t.Parallel()

	cfg := &config.EndpointConfig{
		Name:          "lowest-priority",
		URL:           "http://localhost:11434",
		CheckInterval: 5 * time.Second,
		CheckTimeout:  2 * time.Second,
		Priority:      ptrInt(0),
	}

	applyEndpointDefaults(cfg)

	if cfg.Priority == nil || *cfg.Priority != 0 {
		t.Errorf("Priority = %v, want 0 — explicit zero must not be replaced with the default", cfg.Priority)
	}
}

// TestApplyEndpointDefaults_PartialDefaults verifies that only the zero-value fields
// receive defaults when the config is partially specified.
func TestApplyEndpointDefaults_PartialDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		cfg          config.EndpointConfig
		wantInterval time.Duration
		wantTimeout  time.Duration
		wantPriority int
	}{
		{
			name: "only CheckInterval set",
			cfg: config.EndpointConfig{
				CheckInterval: 15 * time.Second,
				// CheckTimeout and Priority are zero
			},
			wantInterval: 15 * time.Second,
			wantTimeout:  DefaultCheckTimeout,
			wantPriority: DefaultPriority,
		},
		{
			name: "only CheckTimeout set",
			cfg: config.EndpointConfig{
				CheckTimeout: 1 * time.Second,
				// CheckInterval and Priority are zero
			},
			wantInterval: DefaultCheckInterval,
			wantTimeout:  1 * time.Second,
			wantPriority: DefaultPriority,
		},
		{
			name: "only Priority set",
			cfg: config.EndpointConfig{
				Priority: ptrInt(200),
				// CheckInterval and CheckTimeout are zero
			},
			wantInterval: DefaultCheckInterval,
			wantTimeout:  DefaultCheckTimeout,
			wantPriority: 200,
		},
		{
			name: "CheckInterval and Priority set, CheckTimeout zero",
			cfg: config.EndpointConfig{
				CheckInterval: 20 * time.Second,
				Priority:      ptrInt(75),
			},
			wantInterval: 20 * time.Second,
			wantTimeout:  DefaultCheckTimeout,
			wantPriority: 75,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := tc.cfg // copy to avoid mutation across subtests
			applyEndpointDefaults(&cfg)

			if cfg.CheckInterval != tc.wantInterval {
				t.Errorf("CheckInterval = %v, want %v", cfg.CheckInterval, tc.wantInterval)
			}
			if cfg.CheckTimeout != tc.wantTimeout {
				t.Errorf("CheckTimeout = %v, want %v", cfg.CheckTimeout, tc.wantTimeout)
			}
			if cfg.Priority == nil || *cfg.Priority != tc.wantPriority {
				t.Errorf("Priority = %v, want %d", cfg.Priority, tc.wantPriority)
			}
		})
	}
}

// TestLoadFromConfig_WithMinimalEndpoints verifies that endpoints omitting timing fields
// in YAML (which arrive as zero values) successfully load and pass validation.
// This exercises the applyEndpointDefaults → validateEndpointConfig path end-to-end.
func TestLoadFromConfig_WithMinimalEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       config.EndpointConfig
		expectErr bool
	}{
		{
			name: "omitted check_interval and check_timeout receive defaults",
			cfg: config.EndpointConfig{
				Name: "minimal-ollama",
				URL:  "http://localhost:11434",
				Type: "ollama",
				// CheckInterval and CheckTimeout left at zero — applyEndpointDefaults must fill them
