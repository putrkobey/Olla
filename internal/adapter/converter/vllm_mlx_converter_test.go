package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestVLLMMLXConverter_GetFormatName(t *testing.T) {
	t.Parallel()
	c := NewVLLMMLXConverter()
	assert.Equal(t, constants.ProviderTypeVLLMMLX, c.GetFormatName())
}

func TestVLLMMLXConverter_ConvertToFormat(t *testing.T) {
	t.Parallel()

	c := NewVLLMMLXConverter()

	t.Run("converts model with vllm-mlx alias", func(t *testing.T) {
		t.Parallel()
		models := []*domain.UnifiedModel{
			{
				ID:     "mlx-community/Llama-3.2-3B-Instruct-4bit",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "mlx-community/Llama-3.2-3B-Instruct-4bit", Source: constants.ProviderTypeVLLMMLX},
				},
			},
		}

		result, err := c.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

		response, ok := result.(VLLMMLXResponse)
		require.True(t, ok)
		assert.Equal(t, "list", response.Object)
		require.Len(t, response.Data, 1)

		m := response.Data[0]
		assert.Equal(t, "mlx-community/Llama-3.2-3B-Instruct-4bit", m.ID)
		assert.Equal(t, "model", m.Object)
		assert.NotZero(t, m.Created)
		// namespace "mlx-community" extracted from "mlx-community/Llama-3.2-3B-Instruct-4bit"
		assert.Equal(t, "mlx-community", m.OwnedBy)
	})

	t.Run("falls back to first alias when no vllm-mlx alias", func(t *testing.T) {
		t.Parallel()
		models := []*domain.UnifiedModel{
			{
				ID:     "mlx-community/Llama-3.2-3B-Instruct-4bit",
				Family: "llama",
				Aliases: []domain.AliasEntry{
					{Name: "llama3.2:3b-instruct", Source: "ollama"},
				},
			},
		}

		result, err := c.ConvertToFormat(models, ports.ModelFilters{})
		require.NoError(t, err)

