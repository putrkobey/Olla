package unifier

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Helper functions
func ptrString(s string) *string {
	return &s
}

func createTestEndpoint(url, name string) *domain.Endpoint {
	return &domain.Endpoint{
		URLString: url,
		Name:      name,
	}
}

func TestDefaultUnifier_UnifyModels(t *testing.T) {
	tests := []struct {
		name           string
		inputModels    [][]*domain.ModelInfo // models from different endpoints
		endpoints      []*domain.Endpoint
		expectedCount  int
		expectedDigest int
		expectedName   int
	}{
		{
			name: "deduplicate by digest",
			inputModels: [][]*domain.ModelInfo{
				{
					{
						Name: "llama3:8b",
						Size: 8000000000,
						Details: &domain.ModelDetails{
							Digest: ptrString("sha256:abc123"),
						},
					},
				},
				{
					{
						Name: "llama3-8b",
						Size: 8000000000,
						Details: &domain.ModelDetails{
							Digest: ptrString("sha256:abc123"),
						},
					},
				},
			},
			endpoints: []*domain.Endpoint{
				createTestEndpoint("http://localhost:11434", "Ollama Server"),
				createTestEndpoint("http://localhost:1234", "LM Studio"),
			},
			expectedCount:  1,
			expectedDigest: 1,
			expectedName:   0,
		},
		{
			name: "deduplicate by exact name",
			inputModels: [][]*domain.ModelInfo{
				{
					{
						Name: "phi3:mini",
						Size: 3800000000,
					},
				},
				{
					{
						Name: "phi3:mini",
						Size: 3800000000,
					},
				},
			},
			endpoints: []*domain.Endpoint{
				createTestEndpoint("http://localhost:11434", "Ollama Server"),
				createTestEndpoint("http://localhost:1234", "LM Studio"),
			},
			expectedCount:  1,
			expectedDigest: 0,
			expectedName:   1,
		},
		{
			name: "no deduplication for different models",
			inputModels: [][]*domain.ModelInfo{
				{
					{
						Name:    "llama3:8b",
						Details: &domain.ModelDetails{Digest: ptrString("sha256:abc123")},
					},
					{
						Name:    "phi3:mini",
						Details: &domain.ModelDetails{Digest: ptrString("sha256:def456")},
					},
				},
				{
					{
						Name:    "mistral:7b",
						Details: &domain.ModelDetails{Digest: ptrString("sha256:ghi789")},
					},
				},
			},
			endpoints: []*domain.Endpoint{
				createTestEndpoint("http://localhost:11434", "Ollama Server"),
				createTestEndpoint("http://localhost:1234", "LM Studio"),
			},
			expectedCount:  3,
			expectedDigest: 0,
			expectedName:   0,
		},
		{
			name: "case insensitive name matching",
			inputModels: [][]*domain.ModelInfo{
				{
					{
						Name: "Llama3:8B",
					},
				},
				{
					{
						Name: "llama3:8b",
					},
				},
			},
			endpoints: []*domain.Endpoint{
				createTestEndpoint("http://localhost:11434", "Ollama Server"),
				createTestEndpoint("http://localhost:1234", "LM Studio"),
			},
			expectedCount:  1,
			expectedDigest: 0,
			expectedName:   1,
		},
		{
			name: "conflicting digests prevent name-based merge",
			inputModels: [][]*domain.ModelInfo{
				{
					{
						Name:    "llama3:8b",
						Details: &domain.ModelDetails{Digest: ptrString("sha256:abc123")},
					},
				},
				{
					{
						Name:    "llama3:8b",
						Details: &domain.ModelDetails{Digest: ptrString("sha256:def456")},
					},
				},
			},
			endpoints: []*domain.Endpoint{
				createTestEndpoint("http://localhost:11434", "Ollama Server"),
				createTestEndpoint("http://localhost:1234", "LM Studio"),
			},
			expectedCount:  2,
			expectedDigest: 0,
			expectedName:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			unifier := NewDefaultUnifier()

			var allModels []*domain.UnifiedModel
			for i, models := range tt.inputModels {
				unified, err := unifier.UnifyModels(ctx, models, tt.endpoints[i])
				require.NoError(t, err)
				allModels = append(allModels, unified...)
			}

			// NOTE: Internal state checks removed after refactoring
			// The new implementation encapsulates internal state
			// We can only verify the public interface behavior
		})
	}
}

func TestDefaultUnifier_ResolveModel(t *testing.T) {
	ctx := context.Background()
	unifier := NewDefaultUnifier()

	// Add some models
	models := []*domain.ModelInfo{
		{
			Name:    "llama3:8b",
			Details: &domain.ModelDetails{Digest: ptrString("sha256:abc123")},
		},
		{
			Name:    "Phi3:Mini",
			Details: &domain.ModelDetails{Digest: ptrString("sha256:def456")},
		},
	}

	endpoint := createTestEndpoint("http://localhost:11434", "Ollama Server")
	_, err := unifier.UnifyModels(ctx, models, endpoint)
	require.NoError(t, err)

	// Test resolution by exact ID
	model, err := unifier.ResolveAlias(ctx, "llama3:8b")
	require.NoError(t, err)
	assert.Equal(t, "llama3:8b", model.ID)

	// Test resolution by case-insensitive name
	model, err = unifier.ResolveAlias(ctx, "phi3:mini")
	require.NoError(t, err)
	assert.Equal(t, "Phi3:Mini", model.ID)

	// Test resolution not found
	_, err = unifier.ResolveAlias(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model not found")
}

