// Package config_docs holds the validation harness for the YAML snippets that
// appear in docs/content/configuration/dashboard.md. It is a Go test rather
// than a script because Olla silently ignores unknown YAML keys - a typo'd
// dashboard.access_policy.allowed_xidrs would otherwise round-trip through
// the loader without complaint, leaving the docs wrong and the reader none
// the wiser.
//
// Each test case is the literal YAML from the docs page, fed through the same
// yaml.Unmarshal-onto-DefaultConfig path the real loader uses, then Validate.
// A test failure here means the docs promise a key the schema does not accept
// or a value it rejects.
package config_docs

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/config"
)

// runSnippet loads one YAML block onto a fresh DefaultConfig and runs the same
// Validate the real startup runs. Failures here are docs bugs, not config bugs.
func runSnippet(t *testing.T, name, yamlStr string) {
	t.Helper()

	cfg := config.DefaultConfig()
	if err := yaml.Unmarshal([]byte(yamlStr), cfg); err != nil {
		t.Fatalf("%s: yaml parse failed (unknown keys would silently pass; a structural error means the snippet is malformed): %v", name, err)
	}
	if err := cfg.Dashboard.Validate(); err != nil {
		t.Fatalf("%s: dashboard Validate failed: %v", name, err)
	}
	if !cfg.Dashboard.Enabled {
		t.Fatalf("%s: dashboard.enabled did not parse to true", name)
	}
}

// TestDocsSnippet_Disabled is the one docs snippet where Enabled is false.
// Validate must short-circuit clean without forcing the allowlists to be
// populated, so an operator can drop the dashboard with a one-liner.
func TestDocsSnippet_Disabled(t *testing.T) {
	cfg := config.DefaultConfig()
	const yamlStr = `dashboard:
  enabled: false
`
	if err := yaml.Unmarshal([]byte(yamlStr), cfg); err != nil {
		t.Fatalf("disabled snippet: yaml parse failed: %v", err)
	}
	if cfg.Dashboard.Enabled {
		t.Fatalf("disabled snippet: dashboard.enabled parsed as true, want false")
	}
	if err := cfg.Dashboard.Validate(); err != nil {
		t.Fatalf("disabled snippet: Validate should short-circuit, got: %v", err)
	}
}

// TestDocsSnippet_Default is the shipped loopback-only default shown in the
// docs as both the literal default and the "enable and disable" intro example.
// It mirrors DefaultConfig: loopback CIDRs and "localhost" in allowed_hosts
// (localhost is a hostname, not an IP literal, so the auto-accept for
// IP-literal Hosts does not cover it; it's in the default so a no-config
// install is not 403'd against Olla's shipped 0.0.0.0 bind).
func TestDocsSnippet_Default(t *testing.T) {
	const yamlStr = `dashboard:
  enabled: true
  access_policy:
    allowed_cidrs:
      - "127.0.0.0/8"
      - "::1/128"
    allowed_hosts:
      - "localhost"
