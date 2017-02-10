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
	} `yaml:"characteristics"`

	Resources struct {
		Quantization struct {
			Multipliers map[string]float64 `yaml:"multipliers"`
		} `yaml:"quantization"`
		ModelSizes        []ModelSizePattern        `yaml:"model_sizes"`
		ConcurrencyLimits []ConcurrencyLimitPattern `yaml:"concurrency_limits"`
		Defaults          ResourceRequirements      `yaml:"defaults"`
		TimeoutScaling    TimeoutScaling            `yaml:"timeout_scaling"`
	} `yaml:"resources"`

	// PathIndices allows configuring which paths serve specific purposes
	PathIndices struct {
		Health          int `yaml:"health"`
		Models          int `yaml:"models"`
		Completions     int `yaml:"completions"`
		ChatCompletions int `yaml:"chat_completions"`
		Embeddings      int `yaml:"embeddings"`
	} `yaml:"path_indices"`
}

// AuthHint describes the authentication capabilities of a backend profile.
// Fields are intentionally optional. Absent means "we don't know" or "not applicable".
// This is advisory only; the actual auth configuration lives in the endpoint config.
type AuthHint struct {
	// DefaultHeader is the expected credential header for api_key auth.
	// Empty means the backend uses the standard Authorization header.
	DefaultHeader string `yaml:"default_header,omitempty" json:"default_header,omitempty"`

	// Types lists the authentication schemes this backend supports (e.g. ["bearer"]).
	// An empty slice means the backend has no documented auth support.
	Types []string `yaml:"types,omitempty" json:"types,omitempty"`

	// Required indicates whether the backend is known to require authentication.
	// False (default) means auth is optional or not applicable for most deployments.
	Required bool `yaml:"required,omitempty" json:"required,omitempty"`
}

// ModelSizePattern defines resource requirements for models matching specific patterns
type ModelSizePattern struct {
	Patterns            []string `yaml:"patterns"`
	MinMemoryGB         float64  `yaml:"min_memory_gb"`
	RecommendedMemoryGB float64  `yaml:"recommended_memory_gb"`
	MinGPUMemoryGB      float64  `yaml:"min_gpu_memory_gb"`
	EstimatedLoadTimeMS int      `yaml:"estimated_load_time_ms"`
}

// ConcurrencyLimitPattern defines how many concurrent requests a model can handle based on its memory requirements
type ConcurrencyLimitPattern struct {
	MinMemoryGB   float64 `yaml:"min_memory_gb"`
	MaxConcurrent int     `yaml:"max_concurrent"`
}

// TimeoutScaling configures dynamic timeout adjustment based on model characteristics
type TimeoutScaling struct {
	BaseTimeoutSeconds int  `yaml:"base_timeout_seconds"`
	LoadTimeBuffer     bool `yaml:"load_time_buffer"` // adds estimated_load_time_ms to timeout
}

// ContextPattern maps model name patterns to context window sizes
type ContextPattern struct {
	Pattern string `yaml:"pattern"`
	Context int64  `yaml:"context"`
}

// Limitation name constants for features that may be absent on a given backend.
// The passthrough endpoint filter uses exact-name matching against these values,
// so token-counting limitations (which use different names) are never affected.
