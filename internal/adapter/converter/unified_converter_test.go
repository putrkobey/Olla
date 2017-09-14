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
