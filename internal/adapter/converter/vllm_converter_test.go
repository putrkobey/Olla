package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestVLLMConverter_ConvertToFormat(t *testing.T) {
	converter := NewVLLMConverter()

	t.Run("converts unified models to vLLM format with extended metadata", func(t *testing.T) {
		maxContext := int64(2048)
		models := []*domain.UnifiedModel{
			{
				ID:               "tinyllama/1.1b:chat-v1.0",
				Family:           "tinyllama",
				Variant:          "1.1b",
				ParameterSize:    "1.1b",
				ParameterCount:   1100000000,
				Format:           "safetensors",
				MaxContextLength: &maxContext,
				Aliases: []domain.AliasEntry{
					{Name: "TinyLlama/TinyLlama-1.1B-Chat-v1.0", Source: "vllm"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{
						EndpointURL:  "http://192.168.0.1:8000",
						EndpointName: "vllm-server",
						NativeName:   "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
						State:        "loaded",
					},
				},
				Capabilities: []string{"chat"},
			},
			{
				ID:               "llama/3.1:8b-instruct",
				Family:           "llama",
				Variant:          "3.1",
				ParameterSize:    "8b",
				ParameterCount:   8000000000,
				Format:           "safetensors",
				MaxContextLength: &[]int64{131072}[0],
				Aliases: []domain.AliasEntry{
					{Name: "meta-llama/Meta-Llama-3.1-8B-Instruct", Source: "vllm"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{
						EndpointURL:  "http://192.168.0.1:8000",
						EndpointName: "vllm-server",
						NativeName:   "meta-llama/Meta-Llama-3.1-8B-Instruct",
						State:        "available",
					},
				},
				Capabilities: []string{"chat", "completion"},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
