package unifier

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/logger"
)

// ModelUnificationConfig represents the model unification configuration.
// This reads from models.yaml and handles model name normalization across
// different platforms (e.g., "llama-3.1-8b" vs "meta-llama-3.1-8b-instruct").
// Not to be confused with profile configs which define platform behavior.
type ModelUnificationConfig struct {
	ModelExtraction struct {
		ArchitectureMappings map[string]string `yaml:"architecture_mappings"`
		FamilyAliases        map[string]string `yaml:"family_aliases"`
		PublisherMappings    map[string]string `yaml:"publisher_mappings"`
		FamilyPatterns       []PatternConfig   `yaml:"family_patterns"`
	} `yaml:"model_extraction"`

	Quantization struct {
		Mappings map[string]string `yaml:"mappings"`
	} `yaml:"quantization"`

	Capabilities struct {
		TypeCapabilities  map[string][]string `yaml:"type_capabilities"`
		ContextThresholds map[string]int64    `yaml:"context_thresholds"`
		NamePatterns      []NamePatternConfig `yaml:"name_patterns"`
	} `yaml:"capabilities"`

	SpecialRules struct {
		PreserveFamily []string `yaml:"preserve_family"`
		GenericNames   []string `yaml:"generic_names"`
	} `yaml:"special_rules"`
}

// PatternConfig represents a pattern configuration
type PatternConfig struct {
	regex       *regexp.Regexp
	Pattern     string `yaml:"pattern"`
	Description string `yaml:"description"`

	FamilyGroup  int `yaml:"family_group"`
	VariantGroup int `yaml:"variant_group"`
}

// NamePatternConfig represents capability name patterns
type NamePatternConfig struct {
	regex        *regexp.Regexp
	Pattern      string   `yaml:"pattern"`
	Capabilities []string `yaml:"capabilities"`
}

var (
	configInstance *ModelUnificationConfig
	configOnce     sync.Once
	errConfig      error

	// configSource is the path models.yaml was loaded from, or "" when
	// embedded defaults are in use. configParseWarnings records any
	// candidate paths that were found but failed to parse, so a startup
	// diagnostic can explain *why* defaults were used rather than leaving
	// it silent (see issue #204).
	configSource        string
	configParseWarnings []string
)

// LoadModelConfig loads the model unification configuration
func LoadModelConfig() (*ModelUnificationConfig, error) {
	configOnce.Do(func() {
		configInstance, configSource, configParseWarnings = loadConfigFromFile()
		if configInstance == nil {
			return
		}

		if err := configInstance.compilePatterns(); err != nil {
			// the candidate parsed as valid YAML but at least one of its
			// patterns isn't a valid regex. Don't hand back an uncompiled
			// config - regex-driven extraction nil-guards on that, so it
			// would silently no-op rather than panic, which is exactly the
			// #204 class of silent failure. Record why and recover onto the
			// embedded defaults, which are known-good.
			if configSource != "" {
				configParseWarnings = append(configParseWarnings, fmt.Sprintf("%s: %v", configSource, err))
			}
			configSource = ""
			configInstance = getDefaultConfig()
			// defaults are hardcoded and covered by TestPatternCompilation,
			// so this should never fail - but never panic on the assumption,
			// so keep whatever it returns rather than assuming success.
			errConfig = configInstance.compilePatterns()
			return
		}

		errConfig = nil
	})
	return configInstance, errConfig
}

// ConfigSource returns the path models.yaml was loaded from, triggering the
// load if it hasn't happened yet. An empty string means embedded defaults
// are in use - check the returned warnings via LogConfigStatus to see why.
func ConfigSource() string {
	_, _ = LoadModelConfig()
	return configSource
}

// ConfigWarnings returns any parse warnings recorded while loading
// models.yaml (candidates that were found but failed to parse, were
// effectively empty, or had a pattern that didn't compile as regex),
// triggering the load if it hasn't happened yet. Empty means nothing went
// wrong - either a real file loaded cleanly, or no candidate file existed at
// all. Exposed for callers like --validate-config that need the detail
// without depending on log capture.
func ConfigWarnings() []string {
	_, _ = LoadModelConfig()
	return append([]string(nil), configParseWarnings...)
}

// LogConfigStatus reports where the model unification config came from.
// Intended to be called once at startup (rather than relying on the lazy
// first-use load) so a broken config/models.yaml is surfaced immediately
// instead of silently degrading to embedded defaults on first inference
