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
