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
		require.NoError(t, err)

		response, ok := result.(VLLMModelResponse)
		require.True(t, ok)
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 2)

		// Check first model (TinyLlama)
		tinyllama := response.Data[0]
		assert.Equal(t, "TinyLlama/TinyLlama-1.1B-Chat-v1.0", tinyllama.ID)
		assert.Equal(t, "model", tinyllama.Object)
		assert.Equal(t, "TinyLlama", tinyllama.OwnedBy)
		assert.Equal(t, "TinyLlama/TinyLlama-1.1B-Chat-v1.0", tinyllama.Root)
		assert.NotNil(t, tinyllama.MaxModelLen)
		assert.Equal(t, int64(2048), *tinyllama.MaxModelLen)

		// Check permissions are generated
		require.Len(t, tinyllama.Permission, 1)
		perm := tinyllama.Permission[0]
		assert.Equal(t, "model_permission", perm.Object)
		assert.True(t, perm.AllowSampling)
		assert.True(t, perm.AllowLogprobs)
		assert.True(t, perm.AllowView)
		assert.Equal(t, "*", perm.Organization)
		assert.False(t, perm.IsBlocking)

		// Check second model (Llama 3.1)
		llama := response.Data[1]
		assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", llama.ID)
		assert.Equal(t, "meta-llama", llama.OwnedBy)
		assert.NotNil(t, llama.MaxModelLen)
		assert.Equal(t, int64(131072), *llama.MaxModelLen)
	})

	t.Run("handles models without vLLM native names", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:             "phi/4:14.7b-q4km",
				Family:         "phi",
				Variant:        "4",
				ParameterSize:  "14.7b",
				ParameterCount: 14700000000,
				Format:         "gguf",
				Aliases: []domain.AliasEntry{
					{Name: "phi4:latest", Source: "ollama"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{
						EndpointURL: "http://localhost:11434",
						NativeName:  "phi4:latest",
						State:       "loaded",
					},
				},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(VLLMModelResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)

		// Should use first alias when no vLLM native name exists
		model := response.Data[0]
		assert.Equal(t, "phi4:latest", model.ID)
		assert.Equal(t, "vllm", model.OwnedBy) // Default owner when no org in name
		assert.Nil(t, model.MaxModelLen)       // No context length specified
	})

	t.Run("filters work correctly with vLLM format", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:            "tinyllama/1.1b:chat-v1.0",
				Family:        "tinyllama",
				Variant:       "1.1b",
				ParameterSize: "1.1b",
				Aliases: []domain.AliasEntry{
					{Name: "TinyLlama/TinyLlama-1.1B-Chat-v1.0", Source: "vllm"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "TinyLlama/TinyLlama-1.1B-Chat-v1.0"},
				},
			},
			{
				ID:            "llama/3.1:8b",
				Family:        "llama",
				Variant:       "3.1",
				ParameterSize: "8b",
				Aliases: []domain.AliasEntry{
					{Name: "meta-llama/Meta-Llama-3.1-8B-Instruct", Source: "vllm"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "meta-llama/Meta-Llama-3.1-8B-Instruct"},
				},
			},
		}

		filters := ports.ModelFilters{
			Family: "llama",
		}

		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(VLLMModelResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)
		assert.Equal(t, "meta-llama/Meta-Llama-3.1-8B-Instruct", response.Data[0].ID)
	})

	t.Run("generates consistent permissions for all models", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID: "test/model",
				Aliases: []domain.AliasEntry{
					{Name: "org/test-model", Source: "vllm"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "org/test-model"},
				},
			},
		}

		result, err := converter.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

		response := result.(VLLMModelResponse)
