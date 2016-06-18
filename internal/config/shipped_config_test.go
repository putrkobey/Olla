package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// This file holds the dashboard-prefixed tests extracted from
// feature/dashboard-impl's internal/config/shipped_config_test.go. The CB
// tests and the proxyCircuitBreakerNodeFromBytes helper that bundled alongside
// them on that branch are deliberately NOT carried over (simple-dashboard.md
// §5 test dispositions). loadShippedConfig, dashboardNodeFromBytes and
// toYAMLBytes are kept because the three Dashboard tests depend on them.

// repoConfigPath is the path from this package's test working directory
// (internal/config/) to the shipped config.yaml at the repo root.
const repoConfigPath = "../../config/config.yaml"

// repoRootConfigPath is the sibling copy of config.yaml kept at the repo
// root. Load's search order tries config/config.yaml first, so this copy is
// never actually read when config/config.yaml is present - it exists for
// operators who copy just the root file out of a checkout. The drift guard
// below keeps the two byte-identical so which one Load finds never matters.
const repoRootConfigPath = "../../config.yaml"

// TestRootConfig_MatchesShippedConfig is the drift guard between the two
// copies of the default config: config/config.yaml (canonical) and the
// root-level config.yaml (convenience copy for running straight out of a
// checkout). They must stay byte-identical - a divergence here is exactly
// how the F1 blocker happened, where the two copies fell out of sync and
// only one of them got the fix. Mirrors the profile drift guard pattern in
// internal/adapter/registry/profile/builtin_drift_test.go.
func TestRootConfig_MatchesShippedConfig(t *testing.T) {
	t.Parallel()

	shipped, err := os.ReadFile(repoConfigPath)
	require.NoError(t, err, "shipped config/config.yaml must exist")

	root, err := os.ReadFile(repoRootConfigPath)
	require.NoError(t, err, "root config.yaml must exist")

	require.True(t, bytes.Equal(shipped, root),
		"config.yaml drifted from config/config.yaml - keep them byte-identical, edit both together")
}

// loadShippedConfig reads the shipped config/config.yaml via the same
// yaml.Unmarshal-onto-DefaultConfig path the production loader uses. The path
// is resolved against the test's working directory, which go test sets to the
// package directory (internal/config/), so the relative walk lands at the repo
// root's config.yaml regardless of where `go test` is invoked from. Returns the
// decoded Config and the raw bytes so callers can run additional parses.
func loadShippedConfig(t *testing.T) (*Config, []byte) {
