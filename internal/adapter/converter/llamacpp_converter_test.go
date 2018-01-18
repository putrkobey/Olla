package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestLlamaCppConverter_ConvertToFormat(t *testing.T) {
	converter := NewLlamaCppConverter()

	t.Run("converts unified models to llama.cpp format with llamacpp alias", func(t *testing.T) {
		maxContext := int64(8192)
		models := []*domain.UnifiedModel{
			{
				ID:               "llama/3.1:8b-instruct",
				Family:           "llama",
				Variant:          "3.1",
				ParameterSize:    "8b",
				ParameterCount:   8000000000,
				Format:           "gguf",
				MaxContextLength: &maxContext,
				Aliases: []domain.AliasEntry{
					{Name: "llama-3.1-8b-instruct-q4_k_m.gguf", Source: "llamacpp"},
					{Name: "llama3.1:latest", Source: "ollama"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{
						EndpointURL:  "http://localhost:8080",
						EndpointName: "llamacpp-server",
						NativeName:   "llama-3.1-8b-instruct-q4_k_m.gguf",
						State:        "loaded",
					},
				},
				Capabilities: []string{"chat"},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok, "Result should be of type LlamaCppResponse")
		assert.Equal(t, "list", response.Object)
		assert.Len(t, response.Data, 1)

		model := response.Data[0]
		assert.Equal(t, "llama-3.1-8b-instruct-q4_k_m.gguf", model.ID, "Should use llamacpp alias as model ID")
		assert.Equal(t, "model", model.Object)
		assert.NotZero(t, model.Created, "Created timestamp should be set")
		assert.WithinDuration(t, time.Now(), time.Unix(model.Created, 0), 5*time.Second, "Created timestamp should be recent")

		// Owner should be extracted from filename pattern
		assert.Equal(t, "llamacpp", model.OwnedBy, "Should default to llamacpp when no organisation in name")
	})

	t.Run("handles models without llamacpp native names - fallback to first alias", func(t *testing.T) {
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
					{Name: "microsoft/phi-4", Source: "lmstudio"},
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

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)

		model := response.Data[0]
		assert.Equal(t, "phi4:latest", model.ID, "Should use first alias when no llamacpp alias exists")
		assert.Equal(t, "llamacpp", model.OwnedBy, "Should default to llamacpp owner")
	})

	t.Run("handles models without any aliases - fallback to unified ID", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:             "mistral/7b:instruct",
				Family:         "mistral",
				Variant:        "7b",
				ParameterSize:  "7b",
				ParameterCount: 7000000000,
				Format:         "gguf",
				Aliases:        []domain.AliasEntry{}, // No aliases
				SourceEndpoints: []domain.SourceEndpoint{
					{
						EndpointURL: "http://localhost:8080",
						NativeName:  "mistral-7b-instruct",
						State:       "loaded",
					},
				},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)

		model := response.Data[0]
		assert.Equal(t, "mistral/7b:instruct", model.ID, "Should use unified ID when no aliases exist")
	})

	t.Run("filters work correctly with family filter", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:     "tinyllama/1.1b:chat-v1.0",
				Family: "tinyllama",
				Aliases: []domain.AliasEntry{
					{Name: "tinyllama-1.1b-chat.gguf", Source: "llamacpp"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "tinyllama-1.1b-chat.gguf"},
				},
			},
			{
				ID:     "llama/3.1:8b",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "llama-3.1-8b.gguf", Source: "llamacpp"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "llama-3.1-8b.gguf"},
				},
			},
		}

		filters := ports.ModelFilters{
			Family: "llama",
		}

		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1, "Should only include models matching family filter")
		assert.Equal(t, "llama-3.1-8b.gguf", response.Data[0].ID)
	})

	t.Run("handles empty model list", func(t *testing.T) {
		models := []*domain.UnifiedModel{}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Equal(t, "list", response.Object)
		assert.Empty(t, response.Data, "Should return empty data array for empty model list")
	})

	t.Run("handles organisation in path format - slash-separated", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:     "llama/3.1:8b",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "meta-llama/llama-3.1-8b-instruct.gguf", Source: "llamacpp"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "meta-llama/llama-3.1-8b-instruct.gguf"},
				},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)

		model := response.Data[0]
		assert.Equal(t, "meta-llama/llama-3.1-8b-instruct.gguf", model.ID)
		assert.Equal(t, "meta-llama", model.OwnedBy, "Should extract organisation from slash-separated path")
	})

	t.Run("handles hyphenated organisation prefix", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:     "mistral/7b:instruct",
				Family: "mistral",
				Aliases: []domain.AliasEntry{
					{Name: "mistralai-mistral-7b-instruct.gguf", Source: "llamacpp"},
				},
				SourceEndpoints: []domain.SourceEndpoint{
					{NativeName: "mistralai-mistral-7b-instruct.gguf"},
				},
			},
		}

		filters := ports.ModelFilters{}
		result, err := converter.ConvertToFormat(models, filters)
		require.NoError(t, err)

		response, ok := result.(LlamaCppResponse)
		require.True(t, ok)
		assert.Len(t, response.Data, 1)

		model := response.Data[0]
		assert.Equal(t, "mistralai-mistral-7b-instruct.gguf", model.ID)
		// "mistralai" contains "mistral" which is a known organisation
		assert.Equal(t, "mistralai", model.OwnedBy, "Should extract organisation from hyphenated prefix")
	})

	t.Run("all models have consistent structure", func(t *testing.T) {
		models := []*domain.UnifiedModel{
			{
				ID:     "model1",
				Family: "test",
				Aliases: []domain.AliasEntry{
					{Name: "test-model-1.gguf", Source: "llamacpp"},
				},
				SourceEndpoints: []domain.SourceEndpoint{{NativeName: "test-model-1.gguf"}},
			},
			{
				ID:     "model2",
				Family: "test",
				Aliases: []domain.AliasEntry{
					{Name: "test-model-2.gguf", Source: "llamacpp"},
				},
