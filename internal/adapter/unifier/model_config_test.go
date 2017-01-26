package unifier

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/logger"
)

// shippedConfigPath is config/models.yaml, relative to this package
// (internal/adapter/unifier -> repo root is three levels up).
const shippedConfigPath = "../../../config/models.yaml"

// TestShippedConfigParses guards against issue #204: yaml.v3's double-quoted
// scalars only accept a small fixed escape set, so a stray `\d` inside "..."
// breaks the shipped file. loadConfigFromFile() swallows unmarshal errors and
// silently falls back to embedded defaults, so a broken file was never caught
// until now - this test fails loudly instead.
func TestShippedConfigParses(t *testing.T) {
	data, err := os.ReadFile(shippedConfigPath)
	require.NoError(t, err, "config/models.yaml must exist")

	var fromFile ModelUnificationConfig
	require.NoError(t, yaml.Unmarshal(data, &fromFile), "config/models.yaml must be valid YAML")
	require.NoError(t, fromFile.compilePatterns(), "config/models.yaml patterns must compile")
}

// TestShippedConfigMatchesDefaults is an equivalence guard between
// config/models.yaml and getDefaultConfig(). Because loadConfigFromFile()
// silently fell back to defaults on any parse error, the shipped file and
// the embedded fallback drifted apart for years without anyone noticing
// (issue #204). If this fails, reconcile the drift in both directions -
// don't just update one side to make the test pass.
func TestShippedConfigMatchesDefaults(t *testing.T) {
	data, err := os.ReadFile(shippedConfigPath)
	require.NoError(t, err)

	var fromFile ModelUnificationConfig
	require.NoError(t, yaml.Unmarshal(data, &fromFile))

	defaults := getDefaultConfig()

	assert.Equal(t, defaults.ModelExtraction.FamilyPatterns, fromFile.ModelExtraction.FamilyPatterns, "family_patterns drifted")
	assert.Equal(t, defaults.ModelExtraction.ArchitectureMappings, fromFile.ModelExtraction.ArchitectureMappings, "architecture_mappings drifted")
	assert.Equal(t, defaults.ModelExtraction.FamilyAliases, fromFile.ModelExtraction.FamilyAliases, "family_aliases drifted")
	assert.Equal(t, defaults.ModelExtraction.PublisherMappings, fromFile.ModelExtraction.PublisherMappings, "publisher_mappings drifted")
	assert.Equal(t, defaults.Quantization.Mappings, fromFile.Quantization.Mappings, "quantization mappings drifted")
	assert.Equal(t, defaults.Capabilities.TypeCapabilities, fromFile.Capabilities.TypeCapabilities, "type_capabilities drifted")
	assert.Equal(t, defaults.Capabilities.NamePatterns, fromFile.Capabilities.NamePatterns, "capability name_patterns drifted")
	assert.Equal(t, defaults.Capabilities.ContextThresholds, fromFile.Capabilities.ContextThresholds, "context_thresholds drifted")
	assert.ElementsMatch(t, defaults.SpecialRules.PreserveFamily, fromFile.SpecialRules.PreserveFamily, "preserve_family drifted")
	assert.ElementsMatch(t, defaults.SpecialRules.GenericNames, fromFile.SpecialRules.GenericNames, "generic_names drifted")
}

// resetConfigSingleton clears the LoadModelConfig sync.Once cache so a test
// can force a fresh load. Registered via t.Cleanup by every test that
// touches the singleton, so a broken/mutated state from one test can't leak
// into the next (LoadModelConfig's whole point is process-lifetime caching,
// which fights individual test isolation otherwise).
func resetConfigSingleton() {
	configOnce = sync.Once{}
	configInstance, errConfig, configSource, configParseWarnings = nil, nil, "", nil
}

// TestLogConfigStatus_ReportsShippedFileSource verifies LogConfigStatus surfaces
// the real path when config/models.yaml loads cleanly, giving operators a
// startup-log trail rather than the previous total silence.
func TestLogConfigStatus_ReportsShippedFileSource(t *testing.T) {
	dir := filepath.Dir(shippedConfigPath)
	t.Setenv("OLLA_CONFIG_DIR", dir)

	resetConfigSingleton()
	t.Cleanup(resetConfigSingleton)

	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	LogConfigStatus(logger.NewPlainStyledLogger(log))

	assert.Equal(t, filepath.Join(dir, "models.yaml"), ConfigSource())
	assert.Contains(t, buf.String(), "level=INFO")
	assert.Contains(t, buf.String(), "model unification config loaded")
}

// brokenModelsYAML reproduces the exact issue #204 construct: a `\d` inside
// a double-quoted YAML scalar. yaml.v3's double-quoted scalars only accept a
// small fixed escape set, so this is invalid YAML, not just an unwanted regex.
const brokenModelsYAML = `
model_extraction:
  family_patterns:
    - pattern: "^(llama|gemma|phi|qwen)[-_]?(\d+(?:\.\d+)?)"
      family_group: 1
      variant_group: 2
      description: "broken escape, matches issue #204"
