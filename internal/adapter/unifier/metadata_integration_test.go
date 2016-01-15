package unifier

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// parseProfileModels runs a discovery body through the shipped profile's real
// parser, so cases built from it break if parsing and unification drift.
func parseProfileModels(t *testing.T, profileName, body string) []*domain.ModelInfo {
	t.Helper()
	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)
	prof, err := factory.GetProfile(profileName)
	require.NoError(t, err)
	models, err := prof.ParseModelsResponse([]byte(body))
	require.NoError(t, err)
	return models
}

func parseOllamaModels(t *testing.T, body string) []*domain.ModelInfo {
	t.Helper()
	return parseProfileModels(t, domain.ProfileOllama, body)
}

// parseTEIModels feeds a TEI /info body, whose model_type names the real kind.
func parseTEIModels(t *testing.T, body string) []*domain.ModelInfo {
	t.Helper()
	return parseProfileModels(t, constants.ProviderTypeTextEmbeddingsInference, body)
}

// parseLMStudioModels feeds an LM Studio /api/v0/models body, which carries
// "object": "model" beside the real kind in "type".
func parseLMStudioModels(t *testing.T, body string) []*domain.ModelInfo {
	t.Helper()
	return parseProfileModels(t, domain.ProfileLmStudio, body)
}

// TestDefaultUnifier_MetadataExtraction validates metadata extraction
func TestDefaultUnifier_MetadataExtraction(t *testing.T) {
	ctx := context.Background()
	unifier := NewDefaultUnifier()

	tests := []struct {
		name          string
		models        []*domain.ModelInfo
		endpoint      *domain.Endpoint
		expectedModel func(*testing.T, *domain.UnifiedModel)
	}{
		{
			name: "Ollama model with full metadata",
			models: []*domain.ModelInfo{
				{
					Name: "gemma3:12b",
					Size: 8149190253,
					Details: &domain.ModelDetails{
						Family:            ptrString("gemma3"),
						Families:          []string{"gemma3"},
						ParameterSize:     ptrString("12.2B"),
						QuantizationLevel: ptrString("Q4_K_M"),
						Format:            ptrString("gguf"),
						Digest:            ptrString("f4031aab637d1ffa37b42570452ae0e4fad0314754d17ded67322e4b95836f8a"),
						ModifiedAt:        ptrTime(time.Now()),
					},
				},
			},
			endpoint: createTestEndpoint("http://localhost:11434", "Ollama"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Equal(t, "gemma3", model.Family)
				assert.Empty(t, model.Variant)
				assert.Equal(t, "12.2b", model.ParameterSize)
				assert.Equal(t, int64(12200000000), model.ParameterCount)
				assert.Equal(t, "q4km", model.Quantization)
				assert.Equal(t, "gguf", model.Format)
				assert.Contains(t, model.Capabilities, "text-generation")
				assert.Equal(t, "f4031aab637d1ffa37b42570452ae0e4fad0314754d17ded67322e4b95836f8a", model.Metadata["digest"])
				assert.Equal(t, "12.2B", model.Metadata["parameter_size"])
				assert.Equal(t, "Q4_K_M", model.Metadata["quantization_level"])
			},
		},
		{
			name:     "LM Studio model with type and context",
			models:   parseLMStudioModels(t, `{"data":[{"id":"microsoft/phi-4-mini-reasoning","object":"model","type":"llm","publisher":"microsoft","arch":"phi","quantization":"Q4_K_M","state":"not-loaded","max_context_length":131072}]}`),
			endpoint: createTestEndpoint("http://localhost:1234", "LM Studio"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Equal(t, "phi", model.Family)
				assert.Empty(t, model.Variant) // Variant suppressed for metadata-sourced family
				assert.Equal(t, "q4km", model.Quantization)
				assert.Equal(t, int64(131072), *model.MaxContextLength)
				assert.Contains(t, model.Capabilities, "text-generation")
				assert.Contains(t, model.Capabilities, "chat")
				assert.Contains(t, model.Capabilities, "completion")
				assert.Contains(t, model.Capabilities, "reasoning")
				assert.Contains(t, model.Capabilities, "logic")
				assert.Contains(t, model.Capabilities, "long-context")
				assert.Equal(t, "microsoft", model.Metadata["publisher"])
				assert.Equal(t, "llm", model.Metadata["model_type"])
			},
		},
		{
			name:     "Vision model detection",
			models:   parseLMStudioModels(t, `{"data":[{"id":"llava-v1.6-mistral-7b","object":"model","type":"vlm","quantization":"Q5_K_S","max_context_length":4096}]}`),
			endpoint: createTestEndpoint("http://localhost:1234", "LM Studio"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Equal(t, "q5ks", model.Quantization)
				assert.Contains(t, model.Capabilities, "text-generation")
				assert.Contains(t, model.Capabilities, "vision")
				assert.Contains(t, model.Capabilities, "multimodal")
				assert.Contains(t, model.Capabilities, "image-understanding")
				assert.NotContains(t, model.Capabilities, "embeddings")
			},
		},
		{
			// the model name carries no vision hint, so only the real "vlm" type
			// can tag it; "object": "model" must not mask that type
			name:     "LM Studio vlm tagged from its type, not its name",
			models:   parseLMStudioModels(t, `{"data":[{"id":"acme/lookout-7b","object":"model","type":"vlm"}]}`),
			endpoint: createTestEndpoint("http://localhost:1234", "LM Studio"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Contains(t, model.Capabilities, "vision")
				assert.Contains(t, model.Capabilities, "multimodal")
			},
		},
		{
			name: "Code model with instruct",
			models: []*domain.ModelInfo{
				{
					Name: "codellama-34b-instruct",
					Details: &domain.ModelDetails{
						ParameterSize:     ptrString("34B"),
						QuantizationLevel: ptrString("Q3_K_L"),
						Family:            ptrString("codellama"),
					},
				},
			},
			endpoint: createTestEndpoint("http://localhost:11434", "Ollama"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Equal(t, "codellama", model.Family)
				assert.Equal(t, "34b", model.ParameterSize)
				assert.Equal(t, "q3kl", model.Quantization)
				assert.Contains(t, model.Capabilities, "code-generation")
				assert.Contains(t, model.Capabilities, "programming")
				assert.Contains(t, model.Capabilities, "instruction-following")
				assert.Contains(t, model.Capabilities, "chat")
			},
		},
		{
			name:     "LM Studio embedding model",
			models:   parseLMStudioModels(t, `{"data":[{"id":"text-embedding-nomic-embed-text-v1.5","object":"model","type":"embeddings","arch":"nomic-bert","quantization":"Q4_K_M","max_context_length":2048}]}`),
			endpoint: createTestEndpoint("http://localhost:1234", "LM Studio"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Contains(t, model.Capabilities, "embeddings")
				assert.Contains(t, model.Capabilities, "similarity")
				assert.Contains(t, model.Capabilities, "vector-search")
				assert.NotContains(t, model.Capabilities, "text-generation")
			},
		},
		{
			name:     "Ollama embedding-only model typed from capabilities",
			models:   parseOllamaModels(t, `{"models":[{"name":"all-minilm:latest","details":{"family":"bert"},"capabilities":["embedding"]}]}`),
			endpoint: createTestEndpoint("http://localhost:11434", "Ollama"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Contains(t, model.Capabilities, "embeddings")
				assert.Contains(t, model.Capabilities, "similarity")
				assert.Contains(t, model.Capabilities, "vector-search")
				assert.NotContains(t, model.Capabilities, "text-generation")
			},
		},
		{
			name:     "Ollama vision model is a vlm that still generates text",
			models:   parseOllamaModels(t, `{"models":[{"name":"qwen3-vl:8b","details":{"family":"qwen3vl"},"capabilities":["completion","vision","tools"]}]}`),
			endpoint: createTestEndpoint("http://localhost:11434", "Ollama"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Equal(t, "vlm", domain.ModelKindFromMetadata(model.Metadata))
				assert.Contains(t, model.Capabilities, "text-generation", "a vlm is generative, so the kind must not strip it")
				assert.Contains(t, model.Capabilities, "vision")
				assert.Contains(t, model.Capabilities, "multimodal")
				assert.Contains(t, model.Capabilities, "function_calling")
				assert.NotContains(t, model.Capabilities, "embeddings")
			},
		},
		{
			name:     "Ollama tools model has function calling and keeps text-generation",
			models:   parseOllamaModels(t, `{"models":[{"name":"gpt-oss:20b","capabilities":["completion","tools"]}]}`),
			endpoint: createTestEndpoint("http://localhost:11434", "Ollama"),
			expectedModel: func(t *testing.T, model *domain.UnifiedModel) {
				assert.Contains(t, model.Capabilities, "text-generation")
				assert.Contains(t, model.Capabilities, "function_calling")
				assert.NotContains(t, model.Capabilities, "vision")
				assert.NotContains(t, model.Capabilities, "embeddings")
			},
		},
		{
