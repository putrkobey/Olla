package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestUnifiedConverter_ConvertToFormat(t *testing.T) {
	converter := NewUnifiedConverter()

	models := []*domain.UnifiedModel{
		{
			ID:             "llama/3:70b-q4km",
			Family:         "llama",
			Variant:        "3",
			ParameterSize:  "70b",
			ParameterCount: 70000000000,
			Quantization:   "q4km",
			Format:         "gguf",
			Aliases: []domain.AliasEntry{
				{Name: "llama3:latest", Source: "ollama"},
				{Name: "llama3:70b", Source: "generated"},
			},
			SourceEndpoints: []domain.SourceEndpoint{
				{
					EndpointURL:  "http://localhost:11434",
					EndpointName: "Ollama Server",
					NativeName:   "llama3:latest",
					State:        "loaded",
					DiskSize:     40000000000,
				},
			},
			Capabilities:     []string{"chat", "completion"},
			MaxContextLength: int64Ptr(4096),
			PromptTemplateID: "chatml",
		},
		{
			ID:             "phi/4:14.7b-q4km",
			Family:         "phi",
			Variant:        "4",
			ParameterSize:  "14.7b",
			ParameterCount: 14700000000,
			Quantization:   "q4km",
			Format:         "gguf",
			Aliases: []domain.AliasEntry{
				{Name: "phi4:latest", Source: "ollama"},
				{Name: "microsoft/phi-4", Source: "lmstudio"},
			},
			SourceEndpoints: []domain.SourceEndpoint{
				{
					EndpointURL:  "http://localhost:11434",
					EndpointName: "Ollama Server",
					NativeName:   "phi4:latest",
					State:        "not-loaded",
					DiskSize:     8000000000,
				},
				{
					EndpointURL:  "http://localhost:1234",
					EndpointName: "LM Studio",
					NativeName:   "microsoft/phi-4",
					State:        "loaded",
					DiskSize:     8000000000,
				},
			},
			Capabilities: []string{"chat", "code"},
			Metadata: map[string]interface{}{
				"type": "llm",
			},
		},
	}

	t.Run("no filters", func(t *testing.T) {
		filters := ports.ModelFilters{}

		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

