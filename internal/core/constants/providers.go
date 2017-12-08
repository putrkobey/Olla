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

	// Provider display names
	ProviderDisplayOllama                  = "Ollama"
	ProviderDisplayLemonade                = "Lemonade"
	ProviderDisplayLlamaCpp                = "llama.cpp"
	ProviderDisplayLMStudio                = "LM Studio"
	ProviderDisplayOpenAI                  = "OpenAI"
	ProviderDisplayLMDeploy                = "LMDeploy"
	ProviderDisplaySGLang                  = "SGLang"
	ProviderDisplayVLLM                    = "vLLM"
	ProviderDisplayVLLMMLX                 = "vLLM-MLX"
	ProviderDisplayDockerMR                = "Docker Model Runner"
	ProviderDisplayOMLX                    = "oMLX"
	ProviderDisplayTextEmbeddingsInference = "Text Embeddings Inference"

	// Common provider prefixes
	// llama.cpp provider prefixes
	ProviderPrefixLlamaCpp1 = "llamacpp"
	ProviderPrefixLlamaCpp2 = "llama-cpp"
	ProviderPrefixLlamaCpp3 = "llama_cpp"
	ProviderPrefixLMStudio1 = "lmstudio"
	ProviderPrefixLMStudio2 = "lm-studio"
	ProviderPrefixLMStudio3 = "lm_studio"
