package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestNewLMDeployConverter(t *testing.T) {
	t.Parallel()

	c := NewLMDeployConverter()
	assert.NotNil(t, c)
	assert.Equal(t, constants.ProviderTypeLMDeploy, c.GetFormatName())
}

func TestLMDeployConverter_DetermineOwner(t *testing.T) {
	t.Parallel()

	c := NewLMDeployConverter().(*LMDeployConverter)

	cases := []struct {
		modelID  string
		expected string
	}{
		{"internlm/internlm2-chat-7b", "internlm"},
		{"simple-model", constants.ProviderTypeLMDeploy},
		{"no-slash", constants.ProviderTypeLMDeploy},
		// hyphenated known-org fallback (now shared with the other converters
		// via ExtractOwnerFromModelID)
		{"mistralai-mistral-7b-instruct", "mistralai"},
		{"meta-llama-3-8b", "meta"},
	}

	for _, tc := range cases {

		t.Run(tc.modelID, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, c.determineOwner(tc.modelID))
		})
	}
}

func TestLMDeployConverter_ConvertToFormat_Empty(t *testing.T) {
	t.Parallel()

	c := NewLMDeployConverter()
	result, err := c.ConvertToFormat([]*domain.UnifiedModel{}, ports.ModelFilters{})

	require.NoError(t, err)
	resp, ok := result.(profile.LMDeployResponse)
	require.True(t, ok)
	assert.Equal(t, "list", resp.Object)
	assert.Empty(t, resp.Data)
}

func TestLMDeployConverter_ConvertToFormat_SingleModel(t *testing.T) {
	t.Parallel()

	c := NewLMDeployConverter()

	model := &domain.UnifiedModel{
		ID: "internlm/internlm2_5-7b-chat",
		Aliases: []domain.AliasEntry{
			{
				Name:   "internlm/internlm2_5-7b-chat",
				Source: constants.ProviderTypeLMDeploy,
			},
		},
	}

	result, err := c.ConvertToFormat([]*domain.UnifiedModel{model}, ports.ModelFilters{})

	require.NoError(t, err)
	resp, ok := result.(profile.LMDeployResponse)
	require.True(t, ok)
	require.Len(t, resp.Data, 1)

	m := resp.Data[0]
	assert.Equal(t, "internlm/internlm2_5-7b-chat", m.ID)
	assert.Equal(t, "model", m.Object)
	assert.NotZero(t, m.Created)
	// org extracted from ID
