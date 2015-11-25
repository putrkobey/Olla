package converter

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestConverterFactory(t *testing.T) {
	factory := NewConverterFactory()

	t.Run("GetConverter returns correct converters", func(t *testing.T) {
		testCases := []struct {
			format       string
			expectedType string
		}{
			{"unified", "*converter.UnifiedConverter"},
			{"openai", "*converter.OpenAIConverter"},
			{"ollama", "*converter.OllamaConverter"},
			{"lemonade", "*converter.LemonadeConverter"},
			{"llamacpp", "*converter.LlamaCppConverter"},
			{"lmdeploy", "*converter.LMDeployConverter"},
			{"lmstudio", "*converter.LMStudioConverter"},
			{"sglang", "*converter.SGLangConverter"},
			{"vllm", "*converter.VLLMConverter"},
			{"vllm-mlx", "*converter.VLLMMLXConverter"},
			{"docker-model-runner", "*converter.DockerModelRunnerConverter"},
