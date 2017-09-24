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
