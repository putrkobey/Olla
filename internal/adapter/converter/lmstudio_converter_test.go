package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestLMStudioConverter_ConvertToFormat(t *testing.T) {
	converter := NewLMStudioConverter()

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
				{Name: "llama3:latest", Source: "ollama"}, // No LM Studio alias
			},
			SourceEndpoints: []domain.SourceEndpoint{
				{
					EndpointURL: "http://localhost:11434",
					NativeName:  "llama3:latest",
					State:       "loaded",
				},
			},
			Capabilities: []string{"chat", "completion"},
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
					EndpointURL: "http://localhost:11434",
					NativeName:  "phi4:latest",
					State:       "not-loaded",
				},
				{
					EndpointURL: "http://localhost:1234",
					NativeName:  "microsoft/phi-4",
					State:       "loaded",
				},
			},
			Capabilities:     []string{"chat", "code"},
			MaxContextLength: int64Ptr(131072),
			Metadata: map[string]interface{}{
				"type":      "llm",
				"publisher": "microsoft",
			},
		},
		{
			ID:             "qwen/vlm:7b-q4",
			Family:         "qwen",
			Variant:        "vlm",
			ParameterSize:  "7b",
			ParameterCount: 7000000000,
			Quantization:   "q4",
			Format:         "gguf",
			Aliases: []domain.AliasEntry{
