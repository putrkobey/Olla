package constants

const (
	ProviderTypeOllama       = "ollama"
	ProviderTypeLemonade     = "lemonade"
	ProviderTypeLlamaCpp     = "llamacpp"
	ProviderTypeLMStudio     = "lm-studio"
	ProviderTypeOpenAI       = "openai"
	ProviderTypeOpenAICompat = "openai-compatible"
	ProviderTypeLMDeploy     = "lmdeploy"
	ProviderTypeSGLang       = "sglang"
	ProviderTypeVLLM         = "vllm"
	ProviderTypeVLLMMLX      = "vllm-mlx"
	ProviderTypeDockerMR     = "docker-model-runner"
	ProviderTypeOMLX         = "omlx"

	// Text Embeddings Inference (Hugging Face TEI) serves embedding, classifier
	// and reranker models, one per instance, and discovers its model from /info
	// rather than an OpenAI-style model list.
	ProviderTypeTextEmbeddingsInference = "text-embeddings-inference"

