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
			{"", "*converter.UnifiedConverter"}, // Default
		}

		for _, tc := range testCases {
			t.Run(tc.format, func(t *testing.T) {
				converter, err := factory.GetConverter(tc.format)
				require.NoError(t, err)
				assert.NotNil(t, converter)
			})
		}
	})

	t.Run("GetConverter returns error for unsupported format", func(t *testing.T) {
		converter, err := factory.GetConverter("unsupported")
		assert.Nil(t, converter)
		require.Error(t, err)

		qpErr := &ports.QueryParameterError{}
		ok := errors.As(err, &qpErr)
		require.True(t, ok)
		assert.Equal(t, "format", qpErr.Parameter)
		assert.Equal(t, "unsupported", qpErr.Value)
		assert.Contains(t, qpErr.Reason, "unsupported format")
		assert.Contains(t, qpErr.Reason, "unified")
		assert.Contains(t, qpErr.Reason, "openai")
		assert.Contains(t, qpErr.Reason, "ollama")
		assert.Contains(t, qpErr.Reason, "lemonade")
		assert.Contains(t, qpErr.Reason, "llamacpp")
		assert.Contains(t, qpErr.Reason, "lmdeploy")
		assert.Contains(t, qpErr.Reason, "lmstudio")
		assert.Contains(t, qpErr.Reason, "sglang")
		assert.Contains(t, qpErr.Reason, "vllm")
