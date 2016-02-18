package profile

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestConfigurableProfile_GetResourceRequirements(t *testing.T) {
	tests := []struct {
		name      string
		config    *domain.ProfileConfig
		modelName string
		expected  domain.ResourceRequirements
	}{
		{
			name: "70b model with q4 quantization",
			config: &domain.ProfileConfig{
				Resources: struct {
					Quantization struct {
						Multipliers map[string]float64 `yaml:"multipliers"`
					} `yaml:"quantization"`
					ModelSizes        []domain.ModelSizePattern        `yaml:"model_sizes"`
					ConcurrencyLimits []domain.ConcurrencyLimitPattern `yaml:"concurrency_limits"`
					Defaults          domain.ResourceRequirements      `yaml:"defaults"`
					TimeoutScaling    domain.TimeoutScaling            `yaml:"timeout_scaling"`
				}{
					ModelSizes: []domain.ModelSizePattern{
						{
							Patterns:            []string{"70b", "72b"},
							MinMemoryGB:         40,
							RecommendedMemoryGB: 48,
							MinGPUMemoryGB:      40,
							EstimatedLoadTimeMS: 300000,
						},
					},
					Quantization: struct {
						Multipliers map[string]float64 `yaml:"multipliers"`
					}{
						Multipliers: map[string]float64{
							"q4": 0.5,
							"q5": 0.625,
						},
					},
					Defaults: domain.ResourceRequirements{
						MinMemoryGB:         4,
						RecommendedMemoryGB: 8,
						MinGPUMemoryGB:      4,
						RequiresGPU:         false,
						EstimatedLoadTimeMS: 5000,
					},
				},
			},
			modelName: "llama2-70b-q4_K_M",
			expected: domain.ResourceRequirements{
				MinMemoryGB:         20, // 40 * 0.5
				RecommendedMemoryGB: 24, // 48 * 0.5
				MinGPUMemoryGB:      20, // 40 * 0.5
				RequiresGPU:         false,
				EstimatedLoadTimeMS: 300000,
			},
		},
		{
			name: "7b model without quantization",
			config: &domain.ProfileConfig{
				Resources: struct {
					Quantization struct {
						Multipliers map[string]float64 `yaml:"multipliers"`
					} `yaml:"quantization"`
					ModelSizes        []domain.ModelSizePattern        `yaml:"model_sizes"`
					ConcurrencyLimits []domain.ConcurrencyLimitPattern `yaml:"concurrency_limits"`
					Defaults          domain.ResourceRequirements      `yaml:"defaults"`
					TimeoutScaling    domain.TimeoutScaling            `yaml:"timeout_scaling"`
				}{
					ModelSizes: []domain.ModelSizePattern{
						{
							Patterns:            []string{"7b", "8b"},
							MinMemoryGB:         6,
							RecommendedMemoryGB: 8,
							MinGPUMemoryGB:      6,
							EstimatedLoadTimeMS: 30000,
						},
					},
					Defaults: domain.ResourceRequirements{
						MinMemoryGB:         4,
						RecommendedMemoryGB: 8,
						MinGPUMemoryGB:      4,
						RequiresGPU:         false,
						EstimatedLoadTimeMS: 5000,
					},
				},
			},
			modelName: "mistral-7b",
			expected: domain.ResourceRequirements{
				MinMemoryGB:         6,
				RecommendedMemoryGB: 8,
				MinGPUMemoryGB:      6,
				RequiresGPU:         false,
				EstimatedLoadTimeMS: 30000,
			},
		},
		{
			name: "unknown model uses defaults",
			config: &domain.ProfileConfig{
				Resources: struct {
					Quantization struct {
						Multipliers map[string]float64 `yaml:"multipliers"`
					} `yaml:"quantization"`
					ModelSizes        []domain.ModelSizePattern        `yaml:"model_sizes"`
					ConcurrencyLimits []domain.ConcurrencyLimitPattern `yaml:"concurrency_limits"`
					Defaults          domain.ResourceRequirements      `yaml:"defaults"`
					TimeoutScaling    domain.TimeoutScaling            `yaml:"timeout_scaling"`
				}{
					ModelSizes: []domain.ModelSizePattern{
						{
							Patterns:            []string{"7b"},
							MinMemoryGB:         6,
							RecommendedMemoryGB: 8,
							MinGPUMemoryGB:      6,
							EstimatedLoadTimeMS: 30000,
						},
					},
					Defaults: domain.ResourceRequirements{
						MinMemoryGB:         4,
						RecommendedMemoryGB: 8,
						MinGPUMemoryGB:      4,
						RequiresGPU:         true,
