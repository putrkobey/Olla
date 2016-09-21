package domain

// Profile identifiers mirror constants.ProviderType* values intentionally.
// The domain layer must not import internal/core/constants to preserve
// hexagonal isolation.
const (
	ProfileOllama           = "ollama"
	ProfileLemonade         = "lemonade"
	ProfileLlamaCpp         = "llamacpp"
	ProfileLmStudio         = "lm-studio"
	ProfileLMDeploy         = "lmdeploy"
	ProfileSGLang           = "sglang"
	ProfileVLLM             = "vllm"
	ProfileVLLMMLX          = "vllm-mlx"
	ProfileOpenAI           = "openai"
	ProfileOpenAICompatible = "openai-compatible"
	ProfileDockerMR         = "docker-model-runner"
	ProfileOMLX             = "omlx"
	ProfileAuto             = "auto"
)

// PlatformProfile defines how to interact with a specific AI inference platform
type PlatformProfile interface {
	GetName() string
	GetVersion() string

	// GetModelDiscoveryURL returns the URL path for discovering available models
	GetModelDiscoveryURL(baseURL string) string
	// GetHealthCheckPath returns the preferred health check path for this platform
	GetHealthCheckPath() string
	// IsOpenAPICompatible indicates if this platform supports OpenIA-compatible endpoints
	IsOpenAPICompatible() bool
	// GetRequestParsingRules returns rules for extracting model names from requests
