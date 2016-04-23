package domain

import "time"

// InferenceProfile extends PlatformProfile with model-aware capabilities.
// We embed PlatformProfile to avoid breaking thousands of lines of existing code
// while laying the groundwork for smarter routing decisions.
type InferenceProfile interface {
	PlatformProfile

	// Ollama can take 5+ minutes to load a 70B model from disk,
	// while OpenAI typically responds in seconds. This matters for
	// circuit breakers and user experience.
	GetTimeout() time.Duration

	// Documents how many concurrent requests a platform copes with (LM Studio
	// runs 4 in parallel by default since 0.4, vLLM handles hundreds). This is profile
	// metadata only: nothing on the request path enforces it yet, so the
	// backend's own concurrency settings are what limit parallelism today.
	GetMaxConcurrentRequests() int

	// Some platforms have quirks - like requiring specific headers or
	// API key formats. This gives each profile a chance to validate
	// before we waste time on doomed requests.
	ValidateEndpoint(endpoint *Endpoint) error

	// Local instances should generally be preferred over cloud endpoints
	// (lower latency, no egress costs). This provides sensible defaults
	// when users don't explicitly configure priorities.
	GetDefaultPriority() int

	// Escape hatch for platform-specific settings that don't deserve
	// their own interface method. Prevents interface bloat.
	GetConfig() *ProfileConfig

	// Model Capabilities
	GetModelCapabilities(modelName string, registry ModelRegistry) ModelCapabilities
	IsModelSupported(modelName string, registry ModelRegistry) bool
	TransformModelName(fromName string, toFormat string) string

	// Resource Requirements
	GetResourceRequirements(modelName string, registry ModelRegistry) ResourceRequirements
	// Metadata only, like GetMaxConcurrentRequests: not enforced.
	GetOptimalConcurrency(modelName string) int

	// Routing Hints
	GetRoutingStrategy() RoutingStrategy
	ShouldBatchRequests() bool
	GetRequestTimeout(modelName string) time.Duration
}

// ModelCapabilities describes what a model can do.
// This helps us avoid sending vision requests to text-only models,
// or function calls to models that don't support them.
type ModelCapabilities struct {
	ChatCompletion      bool
	TextGeneration      bool
	Embeddings          bool
	Rerank              bool
