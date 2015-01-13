package filter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/filter"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestProfileFilteringIntegration(t *testing.T) {
	tests := []struct {
		name             string
		filterConfig     *domain.FilterConfig
		expectedProfiles []string
	}{
		{
			name: "exclude vllm profile",
			filterConfig: &domain.FilterConfig{
				Exclude: []string{"vllm"},
			},
			expectedProfiles: []string{
				domain.ProfileOllama,
				domain.ProfileLmStudio,
				domain.ProfileOpenAICompatible,
				domain.ProfileLlamaCpp,
			},
		},
		{
			name: "include only ollama and lm-studio",
			filterConfig: &domain.FilterConfig{
				Include: []string{"ollama", "lm-studio"},
			},
			expectedProfiles: []string{
				domain.ProfileOllama,
				domain.ProfileLmStudio,
			},
		},
		{
			name: "include all profiles starting with 'o'",
			filterConfig: &domain.FilterConfig{
				Include: []string{"o*"},
			},
			expectedProfiles: []string{
				domain.ProfileOllama,
				domain.ProfileOpenAICompatible,
			},
		},
		{
			name: "exclude profiles containing 'studio'",
			filterConfig: &domain.FilterConfig{
				Exclude: []string{"*studio*"},
			},
			expectedProfiles: []string{
				domain.ProfileOllama,
				domain.ProfileOpenAICompatible,
				domain.ProfileLlamaCpp,
			},
		},
		{
			name: "include ollama but exclude all",
			filterConfig: &domain.FilterConfig{
				Include: []string{"ollama"},
				Exclude: []string{"*"},
			},
			expectedProfiles: []string{}, // Exclude takes precedence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create profile loader with filter
			loader := profile.NewProfileLoaderWithFilter(
				"testdata/profiles", // Non-existent dir, will use built-ins
				tt.filterConfig,
				filter.NewGlobFilter(),
			)

			// Load profiles
			err := loader.LoadProfiles()
			require.NoError(t, err)

			// Get all loaded profiles
			profiles := loader.GetAllProfiles()

			// Extract profile names
			var profileNames []string
			for name := range profiles {
				profileNames = append(profileNames, name)
			}

			// Check expected profiles are present
			assert.ElementsMatch(t, tt.expectedProfiles, profileNames)
		})
	}
}

func TestModelFilteringIntegration(t *testing.T) {
	ctx := context.Background()

	// Create sample models
	models := []*domain.ModelInfo{
		{Name: "llama3-8b", Type: "chat"},
		{Name: "deepseek-coder-v2", Type: "code"},
		{Name: "deepseek-r1", Type: "reasoning"},
		{Name: "codellama-7b", Type: "code"},
		{Name: "mistral-7b", Type: "chat"},
		{Name: "qwen2-7b", Type: "chat"},
	}

	tests := []struct {
		name          string
		filterConfig  *domain.FilterConfig
		expectedCount int
		expectedNames []string
	}{
		{
			name: "exclude deepseek models",
			filterConfig: &domain.FilterConfig{
				Exclude: []string{"deepseek*"},
			},
			expectedCount: 4,
			expectedNames: []string{"llama3-8b", "codellama-7b", "mistral-7b", "qwen2-7b"},
		},
		{
			name: "include only llama family models",
			filterConfig: &domain.FilterConfig{
				Include: []string{"*llama*"},
			},
			expectedCount: 2,
			expectedNames: []string{"llama3-8b", "codellama-7b"},
		},
		{
			name: "include 7b models",
			filterConfig: &domain.FilterConfig{
				Include: []string{"*-7b"},
			},
			expectedCount: 3,
			expectedNames: []string{"codellama-7b", "mistral-7b", "qwen2-7b"},
		},
		{
			name: "include llama but exclude code models",
			filterConfig: &domain.FilterConfig{
				Include: []string{"*llama*"},
				Exclude: []string{"code*"},
			},
			expectedCount: 1,
			expectedNames: []string{"llama3-8b"},
		},
		{
			name: "complex filter with multiple patterns",
			filterConfig: &domain.FilterConfig{
				Include: []string{"llama*", "mistral*", "qwen*", "code*"},
				Exclude: []string{"*-8b"},
			},
			expectedCount: 3,
			expectedNames: []string{"codellama-7b", "mistral-7b", "qwen2-7b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create filter
			f := filter.NewGlobFilter()

			// Apply filter
			result, err := f.Apply(ctx, tt.filterConfig, models, func(item interface{}) string {
				if model, ok := item.(*domain.ModelInfo); ok {
					return model.Name
				}
				return ""
			})
			require.NoError(t, err)

			// Check count
			assert.Len(t, result.Accepted, tt.expectedCount)

			// Extract filtered model names
			var filteredNames []string
			for _, item := range result.Accepted {
				if model, ok := item.(*domain.ModelInfo); ok {
					filteredNames = append(filteredNames, model.Name)
				}
			}

			// Check expected models
			assert.ElementsMatch(t, tt.expectedNames, filteredNames)
		})
