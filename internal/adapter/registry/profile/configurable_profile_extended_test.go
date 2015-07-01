package profile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestConfigurableProfile_ContextPatterns(t *testing.T) {
	config := &domain.ProfileConfig{
		Name:    "test-profile",
		Version: "1.0",
	}
	config.Models.ContextPatterns = []domain.ContextPattern{
		{Pattern: "*-32k*", Context: 32768},
		{Pattern: "*-16k*", Context: 16384},
		{Pattern: "*-8k*", Context: 8192},
		{Pattern: "*:32k*", Context: 32768},
		{Pattern: "*:16k*", Context: 16384},
		{Pattern: "*:8k*", Context: 8192},
		{Pattern: "llama3*", Context: 8192},
		{Pattern: "gpt-4-turbo*", Context: 128000},
	}
	config.Request.ParsingRules.SupportsStreaming = true

	profile := NewConfigurableProfile(config)

	tests := []struct {
		name            string
		modelName       string
		expectedContext int64
	}{
		{"32k model", "mistral-32k", 32768},
		{"16k model", "claude-16k", 16384},
		{"8k model", "gemma-8k", 8192},
		{"llama3 model", "llama3-70b", 8192},
		{"gpt-4-turbo", "gpt-4-turbo-preview", 128000},
		{"no match defaults to 4096", "some-random-model", 4096},
		{"colon notation 32k", "qwen:32k", 32768},
		{"colon notation 16k", "phi:16k", 16384},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := profile.GetModelCapabilities(tt.modelName, nil)
			assert.Equal(t, tt.expectedContext, caps.MaxContextLength)
		})
	}
}

func TestConfigurableProfile_ConcurrencyLimits(t *testing.T) {
	config := &domain.ProfileConfig{
		Name:    "test-profile",
		Version: "1.0",
	}
	config.Characteristics.MaxConcurrentRequests = 10
	config.Resources.ConcurrencyLimits = []domain.ConcurrencyLimitPattern{
		{MinMemoryGB: 30, MaxConcurrent: 1},
		{MinMemoryGB: 15, MaxConcurrent: 2},
		{MinMemoryGB: 8, MaxConcurrent: 4},
		{MinMemoryGB: 0, MaxConcurrent: 8},
	}
	config.Resources.ModelSizes = []domain.ModelSizePattern{
		{Patterns: []string{"70b"}, MinMemoryGB: 40},
		{Patterns: []string{"30b"}, MinMemoryGB: 20},
		{Patterns: []string{"13b"}, MinMemoryGB: 10},
		{Patterns: []string{"7b"}, MinMemoryGB: 6},
		{Patterns: []string{"3b"}, MinMemoryGB: 3},
	}
	config.Resources.Defaults = domain.ResourceRequirements{
		MinMemoryGB: 4,
	}

	profile := NewConfigurableProfile(config)

	tests := []struct {
		name               string
		modelName          string
		expectedConcurrent int
	}{
		{"70B model gets 1 concurrent", "llama3-70b", 1},
		{"30B model gets 2 concurrent", "falcon-30b", 2},
		{"13B model gets 4 concurrent", "vicuna-13b", 4},
		{"7B model gets 8 concurrent", "mistral-7b", 8},
		{"3B model gets 8 concurrent", "phi-3b", 8},
		{"unknown model uses default", "some-model", 8},
		{"no concurrency limits configured", "any-model", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "no concurrency limits configured" {
				// Test without concurrency limits
				configNoConcurrency := &domain.ProfileConfig{
					Name:    "test-profile",
					Version: "1.0",
				}
				configNoConcurrency.Characteristics.MaxConcurrentRequests = 10
				profileNoConcurrency := NewConfigurableProfile(configNoConcurrency)
				concurrent := profileNoConcurrency.GetOptimalConcurrency(tt.modelName)
				assert.Equal(t, tt.expectedConcurrent, concurrent)
			} else {
				concurrent := profile.GetOptimalConcurrency(tt.modelName)
				assert.Equal(t, tt.expectedConcurrent, concurrent)
			}
		})
	}
}

func TestConfigurableProfile_TimeoutScaling(t *testing.T) {
	config := &domain.ProfileConfig{
		Name:    "test-profile",
		Version: "1.0",
	}
	config.Characteristics.Timeout = 2 * time.Minute
	config.Resources.TimeoutScaling = domain.TimeoutScaling{
		BaseTimeoutSeconds: 30,
		LoadTimeBuffer:     true,
	}
	config.Resources.ModelSizes = []domain.ModelSizePattern{
		{Patterns: []string{"70b"}, EstimatedLoadTimeMS: 300000}, // 5 minutes
		{Patterns: []string{"13b"}, EstimatedLoadTimeMS: 60000},  // 1 minute
		{Patterns: []string{"7b"}, EstimatedLoadTimeMS: 30000},   // 30 seconds
	}
	config.Resources.Defaults = domain.ResourceRequirements{
		EstimatedLoadTimeMS: 5000, // 5 seconds
	}

	profile := NewConfigurableProfile(config)

	tests := []struct {
		name            string
		modelName       string
		expectedTimeout time.Duration
	}{
		{"70B model with load buffer", "llama3-70b", 30*time.Second + 5*time.Minute},
		{"13B model with load buffer", "vicuna-13b", 30*time.Second + 1*time.Minute},
		{"7B model with load buffer", "mistral-7b", 30*time.Second + 30*time.Second},
		{"unknown model uses default", "some-model", 30*time.Second + 5*time.Second},
	}
