package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// mockTranslatorStatsCollector implements ports.StatsCollector for translator stats testing
type mockTranslatorStatsCollector struct {
	translatorStats map[string]ports.TranslatorStats
}

func (m *mockTranslatorStatsCollector) RecordRequest(endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockTranslatorStatsCollector) RecordConnection(endpoint *domain.Endpoint, delta int) {}
func (m *mockTranslatorStatsCollector) RecordSecurityViolation(violation ports.SecurityViolation) {
}
func (m *mockTranslatorStatsCollector) RecordDiscovery(endpoint *domain.Endpoint, success bool, latency time.Duration) {
}
func (m *mockTranslatorStatsCollector) RecordModelRequest(model string, endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockTranslatorStatsCollector) RecordModelError(model string, endpoint *domain.Endpoint, errorType string) {
}
func (m *mockTranslatorStatsCollector) GetModelStats() map[string]ports.ModelStats { return nil }
func (m *mockTranslatorStatsCollector) GetModelEndpointStats() map[string]map[string]ports.EndpointModelStats {
	return nil
}
func (m *mockTranslatorStatsCollector) RecordTranslatorRequest(event ports.TranslatorRequestEvent) {
}
func (m *mockTranslatorStatsCollector) GetProxyStats() ports.ProxyStats { return ports.ProxyStats{} }
func (m *mockTranslatorStatsCollector) GetSecurityStats() ports.SecurityStats {
	return ports.SecurityStats{}
}
func (m *mockTranslatorStatsCollector) GetConnectionStats() map[string]int64     { return nil }
func (m *mockTranslatorStatsCollector) GetConnectionCount(endpoint string) int64 { return 0 }
func (m *mockTranslatorStatsCollector) RecordModelTokens(model string, inputTokens, outputTokens int64) {
}
func (m *mockTranslatorStatsCollector) GetEndpointStats() map[string]ports.EndpointStats {
	return nil
}

func (m *mockTranslatorStatsCollector) GetTranslatorStats() map[string]ports.TranslatorStats {
	if m.translatorStats == nil {
		return make(map[string]ports.TranslatorStats)
	}
	return m.translatorStats
}

// mockTranslatorEndpointRepository for translator stats testing
type mockTranslatorEndpointRepository struct{}

func (m *mockTranslatorEndpointRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}
func (m *mockTranslatorEndpointRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}
func (m *mockTranslatorEndpointRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}
func (m *mockTranslatorEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	return nil
}
func (m *mockTranslatorEndpointRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	return false
}

// mockTranslatorModelRegistry for translator stats testing
type mockTranslatorModelRegistry struct{}

func (m *mockTranslatorModelRegistry) RegisterModel(ctx context.Context, endpointURL string, model *domain.ModelInfo) error {
	return nil
}
func (m *mockTranslatorModelRegistry) RegisterModels(ctx context.Context, endpointURL string, models []*domain.ModelInfo) error {
	return nil
}
func (m *mockTranslatorModelRegistry) GetModelsForEndpoint(ctx context.Context, endpointURL string) ([]*domain.ModelInfo, error) {
	return nil, nil
}
func (m *mockTranslatorModelRegistry) GetEndpointsForModel(ctx context.Context, modelName string) ([]string, error) {
	return nil, nil
}
func (m *mockTranslatorModelRegistry) IsModelAvailable(ctx context.Context, modelName string) bool {
	return false
}
func (m *mockTranslatorModelRegistry) GetAllModels(ctx context.Context) (map[string][]*domain.ModelInfo, error) {
	return nil, nil
}
func (m *mockTranslatorModelRegistry) RemoveEndpoint(ctx context.Context, endpointURL string) error {
	return nil
}
func (m *mockTranslatorModelRegistry) GetStats(ctx context.Context) (domain.RegistryStats, error) {
	return domain.RegistryStats{}, nil
}
func (m *mockTranslatorModelRegistry) ModelsToString(models []*domain.ModelInfo) string {
	return ""
}
func (m *mockTranslatorModelRegistry) ModelsToStrings(models []*domain.ModelInfo) []string {
	return nil
}
func (m *mockTranslatorModelRegistry) GetModelsByCapability(ctx context.Context, capability string) ([]*domain.UnifiedModel, error) {
