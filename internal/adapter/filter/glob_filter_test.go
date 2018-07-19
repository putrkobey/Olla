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
			name: "include only specific profiles",
			config: &domain.FilterConfig{
				Include: []string{"ollama", "lmstudio"},
			},
			items: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
				"vllm":     "profile3",
				"openai":   "profile4",
			},
			expected: map[string]interface{}{
				"ollama":   "profile1",
				"lmstudio": "profile2",
			},
		},
		{
			name: "pattern matching on keys",
			config: &domain.FilterConfig{
				Include: []string{"*studio*", "ollama"},
			},
			items: map[string]interface{}{
				"ollama":    "profile1",
				"lmstudio":  "profile2",
				"vllm":      "profile3",
				"anystudio": "profile4",
			},
			expected: map[string]interface{}{
				"ollama":    "profile1",
				"lmstudio":  "profile2",
				"anystudio": "profile4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewGlobFilter()
			ctx := context.Background()

			result, err := filter.ApplyToMap(ctx, tt.config, tt.items)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGlobFilter_Matches(t *testing.T) {
	tests := []struct {
		name     string
		config   *domain.FilterConfig
		itemName string
		expected bool
	}{
		{
			name:     "nil config matches everything",
			config:   nil,
			itemName: "anything",
			expected: true,
		},
		{
			name:     "empty config matches everything",
			config:   &domain.FilterConfig{},
			itemName: "anything",
			expected: true,
		},
		{
			name: "include pattern matches",
			config: &domain.FilterConfig{
				Include: []string{"llama*"},
			},
			itemName: "llama3",
			expected: true,
		},
		{
			name: "include pattern does not match",
			config: &domain.FilterConfig{
				Include: []string{"llama*"},
			},
			itemName: "phi",
			expected: false,
		},
		{
			name: "exclude pattern blocks match",
			config: &domain.FilterConfig{
				Include: []string{"*"},
				Exclude: []string{"*experimental*"},
			},
			itemName: "model-experimental",
			expected: false,
		},
		{
			name: "multiple include patterns",
			config: &domain.FilterConfig{
				Include: []string{"llama*", "phi*"},
			},
			itemName: "phi-2",
			expected: true,
		},
		{
			name: "exclude overrides include",
			config: &domain.FilterConfig{
				Include: []string{"llama*"},
				Exclude: []string{"llama-uncensored"},
			},
			itemName: "llama-uncensored",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewGlobFilter().(*GlobFilter)
			result := filter.Matches(tt.config, tt.itemName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGlobFilter_PatternMatching(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		pattern  string
		expected bool
	}{
		// Wildcard patterns
		{"match all", "anything", "*", true},
		{"prefix match", "llama3", "llama*", true},
		{"prefix no match", "phi", "llama*", false},
		{"suffix match", "model-7b", "*-7b", true},
		{"suffix no match", "model-13b", "*-7b", false},
		{"contains match", "my-llama-model", "*llama*", true},
		{"contains no match", "phi-model", "*llama*", false},

		// Exact matches
		{"exact match", "llama3", "llama3", true},
		{"exact no match", "llama3.1", "llama3", false},

		// Case insensitivity
		{"case insensitive match", "LLAMA", "llama", true},
		{"case insensitive pattern", "llama", "LLAMA", true},
		{"mixed case", "LLaMa", "llama", true},

		// Complex patterns
		{"double wildcard start end", "test-model-test", "*model*", true},
		{"path-like pattern", "ollama/llama3", "ollama/*", true},
		{"path-like no match", "vllm/llama3", "ollama/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewGlobFilter().(*GlobFilter)
			result := filter.matchesPattern(tt.input, tt.pattern)
			assert.Equal(t, tt.expected, result, "Pattern %s matching %s", tt.pattern, tt.input)
		})
	}
}

func TestGlobFilter_CachePerformance(t *testing.T) {
	filter := NewGlobFilter().(*GlobFilter)

	// First call should cache the result
	result1 := filter.matchesPattern("llama3", "llama*")
	assert.True(t, result1)

	// Second call should use cached result
	result2 := filter.matchesPattern("llama3", "llama*")
	assert.True(t, result2)

	// Check cache has the entry
	filter.cacheMu.RLock()
	cacheSize := len(filter.patternCache)
	filter.cacheMu.RUnlock()
	assert.Positive(t, cacheSize, "Cache should contain entries")

	// Clear cache and verify
	filter.ClearCache()
	filter.cacheMu.RLock()
	cacheSize = len(filter.patternCache)
	filter.cacheMu.RUnlock()
	assert.Equal(t, 0, cacheSize, "Cache should be empty after clear")
}

func TestFilterConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *domain.FilterConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with wildcards",
			config: &domain.FilterConfig{
				Include: []string{"llama*", "*-7b", "*experimental*"},
				Exclude: []string{"*uncensored*"},
			},
			expectError: false,
		},
		{
			name: "empty pattern in include",
			config: &domain.FilterConfig{
				Include: []string{"llama*", ""},
			},
