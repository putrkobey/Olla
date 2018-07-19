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
