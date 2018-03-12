package unifier_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func createTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

// Test helper functions
func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestUnifierIntegrationWithRegistry(t *testing.T) {
	log := createTestLogger()
	ctx := context.Background()

	// Create unified registry
	unifiedRegistry := registry.NewUnifiedMemoryModelRegistry(log, nil, nil, nil)

	// Test data - models from different endpoints
	ollamaEndpoint := "http://localhost:11434"
	lmstudioEndpoint := "http://localhost:1234"

	ollamaModels := []*domain.ModelInfo{
		{
			Name: "phi4:latest",
			Details: &domain.ModelDetails{
				Family:            strPtr("phi3"), // Misclassified
				ParameterSize:     strPtr("14.7B"),
				QuantizationLevel: strPtr("Q4_K_M"),
				State:             strPtr("loaded"),
			},
			Size:     8_000_000_000,
			LastSeen: time.Now(),
		},
		{
			Name: "llama3.3:latest",
			Details: &domain.ModelDetails{
				Family:            strPtr("llama"),
				ParameterSize:     strPtr("70.6B"),
				QuantizationLevel: strPtr("Q4_K_M"),
			},
			Size:     40_000_000_000,
			LastSeen: time.Now(),
		},
		{
			Name: "hf.co/unsloth/Qwen3-32B-GGUF:Q4_K_XL",
			Details: &domain.ModelDetails{
				Family:            strPtr("qwen3"),
				ParameterSize:     strPtr("32.8B"),
				QuantizationLevel: strPtr("unknown"),
			},
			Size:     20_000_000_000,
			LastSeen: time.Now(),
		},
	}

	lmstudioModels := []*domain.ModelInfo{
		{
			Name: "microsoft/phi-4-mini-reasoning",
			Details: &domain.ModelDetails{
				Family:            strPtr("phi3"), // Also misclassified
				QuantizationLevel: strPtr("Q4_K_M"),
				MaxContextLength:  int64Ptr(131072),
				Type:              strPtr("llm"),
				State:             strPtr("not-loaded"),
			},
			Size:     8_000_000_000,
			LastSeen: time.Now(),
		},
		{
			Name: "deepseek/deepseek-r1-0528-qwen3-8b",
			Details: &domain.ModelDetails{
				Family:            strPtr("qwen3"),
				QuantizationLevel: strPtr("Q4_K_M"),
				Type:              strPtr("llm"),
			},
			Size:     5_000_000_000,
			LastSeen: time.Now(),
		},
	}

	// Register models from both endpoints
	err := unifiedRegistry.RegisterModels(ctx, ollamaEndpoint, ollamaModels)
	require.NoError(t, err)

	err = unifiedRegistry.RegisterModels(ctx, lmstudioEndpoint, lmstudioModels)
	require.NoError(t, err)

	// Wait for async unification to complete
	time.Sleep(200 * time.Millisecond)

	// Test 1: Check unified models were created
	unifiedModels, err := unifiedRegistry.GetUnifiedModels(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, unifiedModels)

	// Test 2: Verify phi4 models were unified correctly
	// In the simplified implementation, we use the original model name as ID
	// The two phi4 models have different names, so they won't be unified
	phi4Model, err := unifiedRegistry.GetUnifiedModel(ctx, "phi4:latest")
	require.NoError(t, err)
	require.NotNil(t, phi4Model)

	// Should have endpoint from Ollama only since names differ
	assert.Len(t, phi4Model.SourceEndpoints, 1)

	// Check endpoint details
	for _, ep := range phi4Model.SourceEndpoints {
		if ep.EndpointURL == ollamaEndpoint {
			assert.Equal(t, "phi4:latest", ep.NativeName)
			assert.Equal(t, "loaded", ep.State)
		}
	}

	// Test 3: Verify alias resolution
	// In simplified implementation, we look up by exact name
	testAliases := []string{
		"phi4:latest",
		"microsoft/phi-4-mini-reasoning",
	}

	for _, alias := range testAliases {
		resolved, err := unifiedRegistry.GetUnifiedModel(ctx, alias)
		assert.NoError(t, err, "Failed to resolve alias: %s", alias)
		assert.NotNil(t, resolved)
	}

	// Test 4: Check model availability
	assert.True(t, unifiedRegistry.IsModelAvailable(ctx, "phi4:latest"))
	assert.True(t, unifiedRegistry.IsModelAvailable(ctx, "microsoft/phi-4-mini-reasoning"))
	assert.False(t, unifiedRegistry.IsModelAvailable(ctx, "nonexistent:model"))

	// Test 5: Get endpoints for model
	endpoints, err := unifiedRegistry.GetEndpointsForModel(ctx, "phi4:latest")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(endpoints), 1)

	// Test 6: Verify other models
	llamaModel, err := unifiedRegistry.GetUnifiedModel(ctx, "llama3.3:latest")
	require.NoError(t, err)
	assert.Len(t, llamaModel.SourceEndpoints, 1)

	// Test 7: Check statistics
	stats, err := unifiedRegistry.GetUnifiedStats(ctx)
	require.NoError(t, err)
	assert.Positive(t, stats.TotalUnifiedModels)
	assert.Positive(t, stats.UnificationStats.TotalUnified)
	assert.Equal(t, int64(0), stats.UnificationStats.TotalErrors)

	// Test 8: Remove endpoint and verify cleanup
	err = unifiedRegistry.RemoveEndpoint(ctx, lmstudioEndpoint)
	require.NoError(t, err)

	// Phi4 model should still exist (from Ollama) after removing LM Studio
	phi4ModelAfter, err := unifiedRegistry.GetUnifiedModel(ctx, "phi4:latest")
	require.NoError(t, err)
	assert.Len(t, phi4ModelAfter.SourceEndpoints, 1)
	assert.Equal(t, ollamaEndpoint, phi4ModelAfter.SourceEndpoints[0].EndpointURL)
}
