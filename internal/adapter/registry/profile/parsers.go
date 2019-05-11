package profile

import (
	"fmt"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type ModelResponseParser interface {
	Parse(data []byte) ([]*domain.ModelInfo, error)
}

func getParserForFormat(format string) ModelResponseParser {
	switch format {
	case constants.ProviderTypeOllama:
		return &ollamaParser{}
	case constants.ProviderTypeLemonade:
		return &lemonadeParser{}
	case constants.ProviderPrefixLMStudio1:
		return &lmStudioParser{}
	case constants.ProviderTypeLMDeploy:
		return &lmdeployParser{}
	case constants.ProviderTypeSGLang:
		return &sglangParser{}
	case constants.ProviderTypeVLLM:
		return &vllmParser{}
	case constants.ProviderTypeVLLMMLX:
		return &vllmMLXParser{}
	case constants.ProviderTypeLlamaCpp:
		return &llamaCppParser{}
	case constants.ProviderTypeOpenAI:
		return &openAIParser{}
	case constants.ProviderTypeDockerMR:
		return &dockerModelRunnerParser{}
	case constants.ProviderTypeTextEmbeddingsInference:
		return &textEmbeddingInferenceParser{}
	default:
		// openai format is the de facto standard
		return &openAIParser{}
	}
}

type ollamaParser struct{}

func (p *ollamaParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response OllamaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Ollama response: %w", err)
	}

	models := make([]*domain.ModelInfo, 0, len(response.Models))
	now := time.Now()

