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
