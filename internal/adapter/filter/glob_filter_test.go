package filter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestGlobFilter_Apply(t *testing.T) {
	tests := []struct {
		name         string
		config       *domain.FilterConfig
		items        []string
		expectedPass []string
		expectedFail []string
		expectError  bool
	}{
		{
			name:         "no filter config returns all items",
			config:       nil,
			items:        []string{"model1", "model2", "model3"},
			expectedPass: []string{"model1", "model2", "model3"},
			expectedFail: []string{},
		},
		{
			name:         "empty filter config returns all items",
			config:       &domain.FilterConfig{},
			items:        []string{"model1", "model2", "model3"},
			expectedPass: []string{"model1", "model2", "model3"},
			expectedFail: []string{},
		},
		{
			name: "include all with asterisk",
			config: &domain.FilterConfig{
				Include: []string{"*"},
			},
			items:        []string{"model1", "model2", "model3"},
			expectedPass: []string{"model1", "model2", "model3"},
			expectedFail: []string{},
		},
		{
			name: "exclude specific patterns",
			config: &domain.FilterConfig{
				Include: []string{"*"},
				Exclude: []string{"deepseek*", "*uncensored*"},
			},
			items:        []string{"llama3", "deepseek-coder", "llama-uncensored", "phi"},
			expectedPass: []string{"llama3", "phi"},
			expectedFail: []string{"deepseek-coder", "llama-uncensored"},
		},
		{
			name: "include specific patterns only",
			config: &domain.FilterConfig{
				Include: []string{"llama*", "phi*"},
			},
			items:        []string{"llama3", "deepseek", "phi-2", "mistral"},
			expectedPass: []string{"llama3", "phi-2"},
			expectedFail: []string{"deepseek", "mistral"},
		},
		{
			name: "complex include and exclude",
			config: &domain.FilterConfig{
				Include: []string{"llama*", "phi*", "mistral*"},
				Exclude: []string{"*uncensored*", "*experimental*"},
			},
			items:        []string{"llama3", "llama-uncensored", "phi-2", "phi-experimental", "mistral", "deepseek"},
			expectedPass: []string{"llama3", "phi-2", "mistral"},
			expectedFail: []string{"llama-uncensored", "phi-experimental", "deepseek"},
		},
		{
			name: "suffix matching",
			config: &domain.FilterConfig{
				Include: []string{"*-7b", "*-13b"},
			},
			items:        []string{"llama-7b", "llama-13b", "llama-70b", "phi-2"},
			expectedPass: []string{"llama-7b", "llama-13b"},
			expectedFail: []string{"llama-70b", "phi-2"},
		},
		{
			name: "prefix matching",
			config: &domain.FilterConfig{
				Include: []string{"ollama/*", "lmstudio/*"},
			},
			items:        []string{"ollama/llama3", "lmstudio/phi", "vllm/mistral", "ollama/deepseek"},
			expectedPass: []string{"ollama/llama3", "lmstudio/phi", "ollama/deepseek"},
			expectedFail: []string{"vllm/mistral"},
		},
		{
			name: "contains matching",
			config: &domain.FilterConfig{
				Include: []string{"*llama*"},
			},
			items:        []string{"llama3", "codellama", "my-llama-model", "phi"},
			expectedPass: []string{"llama3", "codellama", "my-llama-model"},
			expectedFail: []string{"phi"},
		},
		{
			name: "case insensitive matching",
			config: &domain.FilterConfig{
				Include: []string{"LLAMA*"},
			},
			items:        []string{"llama3", "Llama3", "LLAMA3", "phi"},
			expectedPass: []string{"llama3", "Llama3", "LLAMA3"},
			expectedFail: []string{"phi"},
		},
		{
			name: "exact match without wildcards",
			config: &domain.FilterConfig{
				Include: []string{"llama3", "phi-2"},
			},
			items:        []string{"llama3", "llama3.1", "phi-2", "phi-2-mini"},
			expectedPass: []string{"llama3", "phi-2"},
			expectedFail: []string{"llama3.1", "phi-2-mini"},
		},
		{
			name: "exclude takes precedence over include",
			config: &domain.FilterConfig{
				Include: []string{"*"},
				Exclude: []string{"llama*"},
			},
			items:        []string{"llama3", "phi", "mistral"},
			expectedPass: []string{"phi", "mistral"},
			expectedFail: []string{"llama3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewGlobFilter()
			ctx := context.Background()

			result, err := filter.Apply(ctx, tt.config, tt.items, func(item interface{}) string {
				return item.(string)
			})

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Extract string values from accepted items
			acceptedStrings := make([]string, 0, len(result.Accepted))
			for _, item := range result.Accepted {
				acceptedStrings = append(acceptedStrings, item.(string))
			}

			// Extract string values from rejected items
			rejectedStrings := make([]string, 0, len(result.Rejected))
			for _, item := range result.Rejected {
				rejectedStrings = append(rejectedStrings, item.(string))
			}

			assert.ElementsMatch(t, tt.expectedPass, acceptedStrings, "Accepted items mismatch")
			assert.ElementsMatch(t, tt.expectedFail, rejectedStrings, "Rejected items mismatch")

			// Verify stats
			assert.Equal(t, len(tt.items), result.Stats.TotalItems)
			assert.Equal(t, len(tt.expectedPass), result.Stats.AcceptedCount)
			assert.Equal(t, len(tt.expectedFail), result.Stats.RejectedCount)
		})
	}
}

func TestGlobFilter_ApplyToMap(t *testing.T) {
	tests := []struct {
		name        string
		config      *domain.FilterConfig
		items       map[string]interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:   "no filter returns all items",
			config: nil,
			items: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
				"vllm":     "profile3",
			},
			expected: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
				"vllm":     "profile3",
			},
		},
		{
			name: "exclude vllm profile",
			config: &domain.FilterConfig{
				Include: []string{"*"},
				Exclude: []string{"vllm"},
			},
			items: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
				"vllm":     "profile3",
			},
			expected: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
			},
		},
		{
