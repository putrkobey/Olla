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
// request.
func LogConfigStatus(log logger.StyledLogger) {
	_, err := LoadModelConfig()
	if log == nil {
		// still trigger the load above so callers that only care about the
		// side effect (warming the singleton) work with a nil logger; there's
		// just nothing to log to
		return
	}
	if err != nil {
		log.Error("model unification config: failed to compile patterns, using embedded defaults", "error", err)
		return
	}

	// Warn about every found-but-unusable candidate even when a later
	// candidate loaded successfully - otherwise a broken ./models.yaml next
	// to a working config/models.yaml goes unmentioned, which is exactly the
	// silence issue #204 complained about. The wording must stay truthful to
	// what actually happened next: only claim "falling back to embedded
	// defaults" when that's genuinely what's active.
	if len(configParseWarnings) > 0 {
		msg := "model unification config: found unusable models.yaml candidate(s), falling back to embedded defaults"
		if configSource != "" {
			msg = fmt.Sprintf("model unification config: found unusable models.yaml candidate(s), using %s instead", configSource)
		}
		log.Warn(msg, "details", strings.Join(configParseWarnings, "; "))
	}

	if configSource != "" {
		log.Info("model unification config loaded", "source", configSource)
		return
	}

	if len(configParseWarnings) == 0 {
		log.Debug("model unification config: no models.yaml found, using embedded defaults")
	}
}

// loadConfigFromFile loads configuration from the YAML file, trying each
// candidate path in turn. Returns the loaded config, the path it came from
// (empty if falling back to defaults), and any parse failures encountered
// along the way for diagnostics.
func loadConfigFromFile() (*ModelUnificationConfig, string, []string) {
	paths := []string{
		"models.yaml",
		"config/models.yaml",
		"config-base/models.yaml",
		"../config/models.yaml",
		"../../config/models.yaml",
	}
	// filepath.Join("", "models.yaml") collapses to "models.yaml", which would
	// silently duplicate the first candidate (and double every warning) when
	// OLLA_CONFIG_DIR is unset - only add it when it's actually set.
	if configDir := os.Getenv("OLLA_CONFIG_DIR"); configDir != "" {
		paths = append(paths, filepath.Join(configDir, "models.yaml"))
	}

	var warnings []string
	seen := make(map[string]bool, len(paths))

	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true

		data, err := os.ReadFile(path)
		if err != nil {
			// a candidate simply not existing is the overwhelmingly common
			// case (most of the search path is speculative) and not worth a
			// warning. Anything else - permission denied, a directory named
			// models.yaml, and so on - is a real problem the operator should
			// hear about, not silence.
			if !errors.Is(err, fs.ErrNotExist) {
				warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			}
			continue
		}

		var config ModelUnificationConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			// found the file but it doesn't parse - record why so LogConfigStatus
			// can tell the operator instead of quietly using defaults
			warnings = append(warnings, fmt.Sprintf("%s: %v", path, err))
			continue
		}

		if config.isEffectivelyEmpty() {
			// an empty or comment-only file unmarshals without error, but
			// treating that as "loaded" repeats the #204 silent-failure class:
			// LogConfigStatus would report success while every lookup against
			// it returns nothing. Warn and let a later candidate (or defaults)
			// win instead.
			warnings = append(warnings, fmt.Sprintf("%s: parsed but contains no configuration", path))
			continue
		}

		config.fillMissingDefaults()
		return &config, path, warnings
	}

	return getDefaultConfig(), "", warnings
}

// fillMissingDefaults backfills keyed capability settings a user file omits.
// A user models.yaml replaces the defaults wholesale, so a file written before
// the reranker and classifier kinds existed would otherwise lose their
// capability tags. context_thresholds needs it too: a missing key reads as 0,
// which would tag every model ultra-long-context. Keys the file does set win.
// Lists and mappings elsewhere are left alone, as an absent entry there reads
// as the operator's choice rather than a gap.
func (c *ModelUnificationConfig) fillMissingDefaults() {
	defaults := getDefaultConfig()

	if c.Capabilities.TypeCapabilities == nil {
		c.Capabilities.TypeCapabilities = make(map[string][]string, len(defaults.Capabilities.TypeCapabilities))
	}
	for kind, caps := range defaults.Capabilities.TypeCapabilities {
		if _, ok := c.Capabilities.TypeCapabilities[kind]; !ok {
			c.Capabilities.TypeCapabilities[kind] = caps
		}
	}

	if c.Capabilities.ContextThresholds == nil {
		c.Capabilities.ContextThresholds = make(map[string]int64, len(defaults.Capabilities.ContextThresholds))
	}
	for name, v := range defaults.Capabilities.ContextThresholds {
		if _, ok := c.Capabilities.ContextThresholds[name]; !ok {
			c.Capabilities.ContextThresholds[name] = v
		}
	}
}

// isEffectivelyEmpty reports whether a successfully-parsed config carries no
// real configuration at all - the signature of an empty or comment-only YAML
// file, which is valid YAML but not a usable models.yaml. Every top-level
// section counts: a file that only sets family_aliases (say) is a legitimate
// partial override and must not be discarded as "empty".
func (c *ModelUnificationConfig) isEffectivelyEmpty() bool {
	return len(c.ModelExtraction.FamilyPatterns) == 0 &&
		len(c.ModelExtraction.FamilyAliases) == 0 &&
		len(c.ModelExtraction.ArchitectureMappings) == 0 &&
		len(c.ModelExtraction.PublisherMappings) == 0 &&
		len(c.Capabilities.NamePatterns) == 0 &&
		len(c.Capabilities.TypeCapabilities) == 0 &&
		len(c.Capabilities.ContextThresholds) == 0 &&
		len(c.Quantization.Mappings) == 0 &&
		len(c.SpecialRules.PreserveFamily) == 0 &&
		len(c.SpecialRules.GenericNames) == 0
}

// compilePatterns compiles all regex patterns in the configuration
func (c *ModelUnificationConfig) compilePatterns() error {
	for i := range c.ModelExtraction.FamilyPatterns {
		regex, err := regexp.Compile(c.ModelExtraction.FamilyPatterns[i].Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile family pattern %s: %w",
				c.ModelExtraction.FamilyPatterns[i].Pattern, err)
		}
		c.ModelExtraction.FamilyPatterns[i].regex = regex
	}

	for i := range c.Capabilities.NamePatterns {
		regex, err := regexp.Compile("(?i)" + c.Capabilities.NamePatterns[i].Pattern)
		if err != nil {
			return fmt.Errorf("failed to compile capability pattern %s: %w",
				c.Capabilities.NamePatterns[i].Pattern, err)
		}
		c.Capabilities.NamePatterns[i].regex = regex
	}

	return nil
}

// getDefaultConfig returns a default configuration if file is not found
func getDefaultConfig() *ModelUnificationConfig {
	config := &ModelUnificationConfig{}

	config.ModelExtraction.FamilyPatterns = []PatternConfig{
		{
			Pattern:      `^(mistral|mixtral)[-_]?(.+)`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Mistral and Mixtral models",
		},
		{
			Pattern:      `^(llama|gemma|phi|qwen)[-_]?(\d+(?:\.\d+)?)`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Common families with version numbers",
		},
		{
			Pattern:      `^[^/]+/(phi|llama|gemma|qwen|mistral|glm|granite|nemotron|exaone|kimi|minimax|hunyuan|olmo|internlm|smollm)(?:[-_.:]?(\d+(?:\.\d+)?))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Models with publisher prefix",
		},
		{
			Pattern:      `^(codellama|starcoder|vicuna|falcon|yi)[-_]?(\d+[bB]?)`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Code and specialised models",
		},
		{
			Pattern:      `^(gpt-oss)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "OpenAI open-weight gpt-oss models",
		},
		{
			Pattern:      `^(gpt)(?:[-_.:]?(2|j|neox))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "GPT variants",
		},
		{
			Pattern:      `^(deepseek)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "DeepSeek models",
		},
		{
			Pattern:      `^(devstral|magistral|codestral|ministral)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Mistral AI code/agent/edge variants (Devstral, Magistral, Codestral, Ministral)",
		},
		{
			Pattern:      `^(glm)(?:[-_.:]?(\d+(?:\.\d+)?))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "GLM models (Zhipu / Z.ai)",
		},
		{
			Pattern:      `^(kimi)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Kimi models (Moonshot AI)",
		},
		{
			Pattern:      `^(granite)(?:[-_.:]?(\d+(?:\.\d+)?))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "IBM Granite models",
		},
		{
			Pattern:      `^(nemotron)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "NVIDIA Nemotron models",
		},
		{
			Pattern:      `^(exaone)(?:[-_.:]?(\d+(?:\.\d+)?))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "LG AI EXAONE models",
		},
		{
			Pattern:      `^(hunyuan)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
			VariantGroup: 2,
			Description:  "Tencent Hunyuan models",
		},
		{
			Pattern:      `^(minimax)(?:[-_.:](.+))?(?:$|[^a-zA-Z])`,
			FamilyGroup:  1,
