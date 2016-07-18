package metrics

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// TestIntegration_OllamaMetricsExtraction verifies the full metrics extraction pipeline
func TestIntegration_OllamaMetricsExtraction(t *testing.T) {
	// Real Ollama response
	ollamaResponse := []byte(`{
		"model": "llama2:latest",
		"created_at": "2024-01-01T00:00:00Z",
		"done": true,
		"total_duration": 5589157167,
		"load_duration": 3013701500,
		"prompt_eval_count": 26,
		"prompt_eval_duration": 2000000000,
		"eval_count": 290,
		"eval_duration": 2575455000
	}`)

	// Create test profile for Ollama
	profile := &mockProfile{
		name: "ollama",
		config: &domain.ProfileConfig{
			Metrics: domain.MetricsConfig{
				Extraction: domain.MetricsExtractionConfig{
					Enabled: true,
					Source:  "response_body",
					Format:  "json",
					Paths: map[string]string{
						"model":              "$.model",
						"done":               "$.done",
						"input_tokens":       "$.prompt_eval_count",
						"output_tokens":      "$.eval_count",
						"prompt_duration_ns": "$.prompt_eval_duration",
						"eval_duration_ns":   "$.eval_duration",
						"total_duration_ns":  "$.total_duration",
						"load_duration_ns":   "$.load_duration",
					},
					Calculations: map[string]string{
						"tokens_per_second": "output_tokens / (eval_duration_ns / 1000000000)",
						"ttft_ms":           "prompt_duration_ns / 1000000",
					},
				},
			},
		},
	}

	factory := &mockProfileFactory{
		profiles: map[string]*mockProfile{
			"ollama": profile,
		},
	}

	// Create extractor with the new implementation
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	testLogger := logger.NewPlainStyledLogger(log)
	extractor, err := NewExtractor(factory, testLogger)
	require.NoError(t, err)

	// Validate profile (pre-compiles expressions)
	err = extractor.ValidateProfile(profile)
	require.NoError(t, err)

	// Test extraction
	ctx := context.Background()
	metrics := extractor.ExtractFromChunk(ctx, ollamaResponse, "ollama")
	require.NotNil(t, metrics)

	// Verify extracted metrics
	assert.Equal(t, "llama2:latest", metrics.Model)
	assert.Equal(t, int32(26), metrics.InputTokens)
	assert.Equal(t, int32(290), metrics.OutputTokens)
	assert.Equal(t, int32(26+290), metrics.TotalTokens)
	assert.True(t, metrics.IsComplete)

	// Verify calculated metrics
	assert.Equal(t, int32(2000), metrics.TTFTMs)           // 2000000000 ns / 1000000 = 2000 ms
	assert.InDelta(t, 112.6, metrics.TokensPerSecond, 0.1) // 290 / 2.575455 ≈ 112.6

