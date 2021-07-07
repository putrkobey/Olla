package domain

import "time"

// ProfileConfig is loaded from YAML files so users can add support for
// new inference platforms without touching Go code. Much easier than
// submitting PRs for every new LLM server that pops up.
type ProfileConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	DisplayName string `yaml:"display_name"`
	Description string `yaml:"description"`

	Detection struct {
		Headers           []string `yaml:"headers"`
		UserAgentPatterns []string `yaml:"user_agent_patterns"`
		ResponsePatterns  []string `yaml:"response_patterns"`
		PathIndicators    []string `yaml:"path_indicators"`
		DefaultPorts      []int    `yaml:"default_ports"`
	} `yaml:"detection"`

	Request struct {
		ModelFieldPaths []string `yaml:"model_field_paths"`
		ResponseFormat  string   `yaml:"response_format"`
		ParsingRules    struct {
			ChatCompletionsPath string `yaml:"chat_completions_path"`
			CompletionsPath     string `yaml:"completions_path"`
			GeneratePath        string `yaml:"generate_path"`
			ModelFieldName      string `yaml:"model_field_name"`
			SupportsStreaming   bool   `yaml:"supports_streaming"`
		} `yaml:"parsing_rules"`
	} `yaml:"request"`

	// Metrics extraction configuration for provider responses
	Metrics MetricsConfig `yaml:"metrics,omitempty"`

	Models struct {
		CapabilityPatterns map[string][]string `yaml:"capability_patterns"`
		NameFormat         string              `yaml:"name_format"`
		ContextPatterns    []ContextPattern    `yaml:"context_patterns"`
	} `yaml:"models"`

	// Routing configuration for URL prefix mapping
	Routing struct {
		Prefixes []string `yaml:"prefixes"`
	} `yaml:"routing"`

	API struct {
		// AnthropicSupport declares whether this backend natively speaks the
		// Anthropic Messages API. When present and enabled, the translator layer
		// can skip the Anthropic-to-OpenAI conversion and forward requests directly.
		// Nil means the backend has no native Anthropic support (the common case).
		AnthropicSupport   *AnthropicSupportConfig `yaml:"anthropic_support,omitempty"`
		ModelDiscoveryPath string                  `yaml:"model_discovery_path"`
		HealthCheckPath    string                  `yaml:"health_check_path"`
		Paths              []string                `yaml:"paths"`
		OpenAICompatible   bool                    `yaml:"openai_compatible"`
	} `yaml:"api"`

	Characteristics struct {
		// Auth declares optional authentication hints for this backend profile.
		// This is purely informational at the profile level and does not enforce
		// validation. Future tooling (startup warnings, status endpoints) can use
		// it to guide operators when configuring endpoint auth.
		Auth                  AuthHint      `yaml:"auth,omitempty" json:"auth,omitempty"`
		Timeout               time.Duration `yaml:"timeout"`
		MaxConcurrentRequests int           `yaml:"max_concurrent_requests"`
		DefaultPriority       int           `yaml:"default_priority"`
		StreamingSupport      bool          `yaml:"streaming_support"`
