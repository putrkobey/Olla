package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// mockProfileFactory provides test profiles
type mockProfileFactory struct {
	profiles map[string]*mockProfile
}

func (f *mockProfileFactory) GetProfile(name string) (domain.InferenceProfile, error) {
	if p, ok := f.profiles[name]; ok {
		return p, nil
	}
	return nil, nil
}

// mockProfile implements InferenceProfile for testing
type mockProfile struct {
	name   string
	config *domain.ProfileConfig
}

// PlatformProfile methods
func (p *mockProfile) GetName() string                            { return p.name }
func (p *mockProfile) GetVersion() string                         { return "1.0" }
func (p *mockProfile) GetModelDiscoveryURL(baseURL string) string { return "/models" }
func (p *mockProfile) GetHealthCheckPath() string                 { return "/" }
func (p *mockProfile) IsOpenAPICompatible() bool                  { return false }
func (p *mockProfile) GetRequestParsingRules() domain.RequestParsingRules {
	return domain.RequestParsingRules{}
}
func (p *mockProfile) GetModelResponseFormat() domain.ModelResponseFormat {
	return domain.ModelResponseFormat{}
}
func (p *mockProfile) GetDetectionHints() domain.DetectionHints {
	return domain.DetectionHints{}
}
func (p *mockProfile) ParseModelsResponse(data []byte) ([]*domain.ModelInfo, error) {
	return nil, nil
}
func (p *mockProfile) GetPaths() []string                      { return nil }
func (p *mockProfile) GetPath(index int) string                { return "/" }
func (p *mockProfile) GetRoutingPrefixes() []string            { return []string{p.name} }
func (p *mockProfile) ExtractModel(r *http.Request) string     { return "" }
func (p *mockProfile) ExtractModelFromPath(path string) string { return "" }
func (p *mockProfile) ExtractModelFromBody(body []byte) string { return "" }

// InferenceProfile methods
func (p *mockProfile) GetTimeout() time.Duration                        { return 30 * time.Second }
func (p *mockProfile) GetMaxConcurrentRequests() int                    { return 10 }
func (p *mockProfile) ValidateEndpoint(endpoint *domain.Endpoint) error { return nil }
func (p *mockProfile) GetDefaultPriority() int                          { return 100 }
func (p *mockProfile) GetConfig() *domain.ProfileConfig                 { return p.config }
func (p *mockProfile) GetModelCapabilities(modelName string, registry domain.ModelRegistry) domain.ModelCapabilities {
	return domain.ModelCapabilities{}
}
func (p *mockProfile) IsModelSupported(modelName string, registry domain.ModelRegistry) bool {
	return true
}
func (p *mockProfile) TransformModelName(fromName string, toFormat string) string {
	return fromName
}
func (p *mockProfile) GetOptimalConcurrency(modelName string) int {
	return 1
}
func (p *mockProfile) GetRequestTimeout(modelName string) time.Duration {
	return 30 * time.Second
}
func (p *mockProfile) GetResourceRequirements(modelName string, registry domain.ModelRegistry) domain.ResourceRequirements {
	return domain.ResourceRequirements{MinMemoryGB: 4}
}
func (p *mockProfile) GetRoutingStrategy() domain.RoutingStrategy {
	return domain.RoutingStrategy{}
}
func (p *mockProfile) ShouldBatchRequests() bool {
	return false
}

func TestExtractor_ExtractMetrics_OllamaResponse(t *testing.T) {
	// Create test profile with Ollama metrics configuration
	ollamaProfile := &mockProfile{
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
						"total_duration_ns":  "$.total_duration",
						"prompt_duration_ns": "$.prompt_eval_duration",
						"eval_duration_ns":   "$.eval_duration",
					},
					Calculations: map[string]string{
						"tokens_per_second": "output_tokens / (eval_duration_ns / 1000000000)",
						"ttft_ms":           "prompt_duration_ns / 1000000",
						"total_ms":          "total_duration_ns / 1000000",
					},
				},
			},
		},
	}

	factory := &mockProfileFactory{
		profiles: map[string]*mockProfile{
			"ollama": ollamaProfile,
		},
	}

	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	testLogger := logger.NewPlainStyledLogger(log)
	extractor, err := NewExtractor(factory, testLogger)
	require.NoError(t, err)

	// Validate the profile
	err = extractor.ValidateProfile(ollamaProfile)
	require.NoError(t, err)

	// Test Ollama response
	ollamaResponse := map[string]interface{}{
		"model":                "llama2:latest",
		"created_at":           "2024-01-01T00:00:00Z",
		"response":             "Hello, world!",
