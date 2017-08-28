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
	t.Helper()

	abs, err := filepath.Abs(repoConfigPath)
	if err != nil {
		t.Fatalf("resolve %s: %v", repoConfigPath, err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read shipped config %s: %v\n"+
			"if you moved config.yaml, update repoConfigPath in this test", abs, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		t.Fatalf("shipped config.yaml failed to parse: %v", err)
	}
	return cfg, data
}

// dashboardNodeFromBytes parses raw YAML and returns the yaml.Node under the
// top-level `dashboard` key. Returns an error if `dashboard` is absent. Used to
// scope a KnownFields(true) check to just the dashboard section: running it on
// the full Config trips on unrelated legacy keys elsewhere in config.yaml that
// are themselves silent-drop bugs but out of scope here.
func dashboardNodeFromBytes(t *testing.T, data []byte) *yaml.Node {
	t.Helper()

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse config.yaml as yaml.Node: %v", err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		t.Fatal("config.yaml parsed to empty document")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		t.Fatal("config.yaml top level is not a mapping")
	}
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == "dashboard" {
			return top.Content[i+1]
		}
	}
	t.Fatal("config.yaml has no top-level `dashboard` key; the shipped default must ship a dashboard block")
	return nil
}

// toYAMLBytes re-encodes a yaml.Node so a scoped KnownFields check can decode
// just that subtree against a fresh struct value.
func toYAMLBytes(t *testing.T, node *yaml.Node) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		t.Fatalf("re-encode dashboard node: %v", err)
	}
	if err := enc.Close(); err != nil {
