package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestOllamaProfile_InferenceProfile(t *testing.T) {
	factory := testFactory(t)
	profile, err := factory.GetProfile(domain.ProfileOllama)
	if err != nil {
		t.Fatalf("Failed to get ollama profile: %v", err)
	}

	t.Run("GetTimeout", func(t *testing.T) {
		timeout := profile.GetTimeout()
		assert.Equal(t, 5*time.Minute, timeout, "Ollama should have 5 minute timeout for large models")
	})

	t.Run("GetMaxConcurrentRequests", func(t *testing.T) {
		max := profile.GetMaxConcurrentRequests()
		assert.Equal(t, 10, max, "Ollama should handle 10 concurrent requests")
	})

	t.Run("GetModelCapabilities", func(t *testing.T) {
		tests := []struct {
			name     string
			model    string
			expected domain.ModelCapabilities
		}{
			{
				name:  "Standard chat model",
				model: "llama3:8b",
				expected: domain.ModelCapabilities{
					ChatCompletion:   true,
					TextGeneration:   true,
					StreamingSupport: true,
					MaxContextLength: 8192,
					MaxOutputTokens:  2048,
					FunctionCalling:  true,
				},
			},
			{
				name:  "Embedding model",
				model: "nomic-embed-text",
				expected: domain.ModelCapabilities{
					ChatCompletion:   false,
					TextGeneration:   false,
					Embeddings:       true,
					StreamingSupport: true,
					MaxContextLength: 4096,
					MaxOutputTokens:  2048,
				},
			},
			{
				name:  "Vision model",
				model: "llava:13b",
				expected: domain.ModelCapabilities{
					ChatCompletion:      true,
					TextGeneration:      true,
					VisionUnderstanding: true,
					StreamingSupport:    true,
					MaxContextLength:    4096,
					MaxOutputTokens:     2048,
				},
			},
			{
				name:  "Code model",
				model: "codellama:34b",
				expected: domain.ModelCapabilities{
					ChatCompletion:   true,
					TextGeneration:   true,
					CodeGeneration:   true,
					StreamingSupport: true,
					MaxContextLength: 4096,
					MaxOutputTokens:  2048,
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				caps := profile.GetModelCapabilities(tt.model, nil)
				assert.Equal(t, tt.expected, caps)
			})
		}
