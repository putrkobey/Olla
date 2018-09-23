package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestDockerModelRunnerConverter_GetFormatName(t *testing.T) {
	t.Parallel()
	c := NewDockerModelRunnerConverter()
	assert.Equal(t, constants.ProviderTypeDockerMR, c.GetFormatName())
}

func TestDockerModelRunnerConverter_ConvertToFormat(t *testing.T) {
	t.Parallel()

	c := NewDockerModelRunnerConverter()

	t.Run("converts model with DMR alias", func(t *testing.T) {
		t.Parallel()
		models := []*domain.UnifiedModel{
			{
				ID:     "llama/3.1:8b-q4km",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "ai/llama-3.1-8b", Source: constants.ProviderTypeDockerMR},
				},
			},
		}

		result, err := c.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

		response, ok := result.(DockerModelRunnerResponse)
		require.True(t, ok)
		assert.Equal(t, "list", response.Object)
		require.Len(t, response.Data, 1)

		m := response.Data[0]
		assert.Equal(t, "ai/llama-3.1-8b", m.ID)
		assert.Equal(t, "model", m.Object)
		assert.NotZero(t, m.Created)
		// namespace "ai" extracted from "ai/llama-3.1-8b"
		assert.Equal(t, "ai", m.OwnedBy)
	})

	t.Run("falls back to first alias when no DMR alias", func(t *testing.T) {
		t.Parallel()
		models := []*domain.UnifiedModel{
			{
				ID:     "llama/3.1:8b-q4km",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "llama3.1:8b-q4_K_M", Source: "ollama"},
				},
			},
		}

		result, err := c.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

		response, ok := result.(DockerModelRunnerResponse)
		require.True(t, ok)
		require.Len(t, response.Data, 1)
		assert.Equal(t, "llama3.1:8b-q4_K_M", response.Data[0].ID)
	})

	t.Run("falls back to unified ID when no aliases at all", func(t *testing.T) {
		t.Parallel()
		models := []*domain.UnifiedModel{
			{
				ID:      "fallback/model",
				Aliases: []domain.AliasEntry{},
			},
		}

		result, err := c.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

		response, ok := result.(DockerModelRunnerResponse)
