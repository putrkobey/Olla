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
