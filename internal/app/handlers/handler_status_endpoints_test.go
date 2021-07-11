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

// mockStatusEndpointRepository for status endpoint testing
type mockStatusEndpointRepository struct {
	endpoints []*domain.Endpoint
}

func (m *mockStatusEndpointRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.endpoints, nil
}

func (m *mockStatusEndpointRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	healthy := make([]*domain.Endpoint, 0)
	for _, ep := range m.endpoints {
		if ep.Status == domain.StatusHealthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy, nil
}

func (m *mockStatusEndpointRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	routable := make([]*domain.Endpoint, 0)
	for _, ep := range m.endpoints {
		if ep.Status == domain.StatusHealthy {
			routable = append(routable, ep)
		}
	}
	return routable, nil
}

func (m *mockStatusEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	for i, ep := range m.endpoints {
		if ep.Name == endpoint.Name {
			m.endpoints[i] = endpoint
			return nil
		}
	}
	return nil
}

func (m *mockStatusEndpointRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	urlStr := endpointURL.String()
	for _, ep := range m.endpoints {
		if ep.URLString == urlStr {
			return true
		}
	}
	return false
}

// mockStatusStatsCollector for status endpoint testing
type mockStatusStatsCollector struct {
	endpointStats      map[string]ports.EndpointStats
	proxyStats         ports.ProxyStats
	modelStats         map[string]ports.ModelStats
	modelEndpointStats map[string]map[string]ports.EndpointModelStats
	connectionStats    map[string]int64
}

func (m *mockStatusStatsCollector) RecordRequest(endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockStatusStatsCollector) RecordConnection(endpoint *domain.Endpoint, delta int)     {}
func (m *mockStatusStatsCollector) RecordSecurityViolation(violation ports.SecurityViolation) {}
func (m *mockStatusStatsCollector) RecordDiscovery(endpoint *domain.Endpoint, success bool, latency time.Duration) {
}
func (m *mockStatusStatsCollector) RecordModelRequest(model string, endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockStatusStatsCollector) RecordModelError(model string, endpoint *domain.Endpoint, errorType string) {
}
func (m *mockStatusStatsCollector) GetModelStats() map[string]ports.ModelStats {
	if m.modelStats == nil {
		return make(map[string]ports.ModelStats)
	}
	return m.modelStats
}
func (m *mockStatusStatsCollector) GetModelEndpointStats() map[string]map[string]ports.EndpointModelStats {
	if m.modelEndpointStats == nil {
		return make(map[string]map[string]ports.EndpointModelStats)
	}
	return m.modelEndpointStats
}
func (m *mockStatusStatsCollector) RecordTranslatorRequest(event ports.TranslatorRequestEvent) {}
func (m *mockStatusStatsCollector) GetTranslatorStats() map[string]ports.TranslatorStats {
	return nil
}
func (m *mockStatusStatsCollector) GetProxyStats() ports.ProxyStats { return m.proxyStats }
func (m *mockStatusStatsCollector) GetSecurityStats() ports.SecurityStats {
	return ports.SecurityStats{}
}
func (m *mockStatusStatsCollector) GetConnectionStats() map[string]int64 {
	if m.connectionStats == nil {
		return make(map[string]int64)
	}
	return m.connectionStats
}
func (m *mockStatusStatsCollector) GetConnectionCount(endpoint string) int64 {
	return m.connectionStats[endpoint]
}
func (m *mockStatusStatsCollector) RecordModelTokens(model string, inputTokens, outputTokens int64) {
}

func (m *mockStatusStatsCollector) GetEndpointStats() map[string]ports.EndpointStats {
	if m.endpointStats == nil {
		return make(map[string]ports.EndpointStats)
	}
	return m.endpointStats
}

// mockStatusModelRegistry for status endpoint testing
type mockStatusModelRegistry struct {
	endpointModels map[string]*domain.EndpointModels
}

func (m *mockStatusModelRegistry) RegisterModel(ctx context.Context, endpointURL string, model *domain.ModelInfo) error {
	return nil
}
func (m *mockStatusModelRegistry) RegisterModels(ctx context.Context, endpointURL string, models []*domain.ModelInfo) error {
	return nil
}
func (m *mockStatusModelRegistry) GetModelsForEndpoint(ctx context.Context, endpointURL string) ([]*domain.ModelInfo, error) {
	return nil, nil
}
func (m *mockStatusModelRegistry) GetEndpointsForModel(ctx context.Context, modelName string) ([]string, error) {
	return nil, nil
}
func (m *mockStatusModelRegistry) IsModelAvailable(ctx context.Context, modelName string) bool {
	return false
}
func (m *mockStatusModelRegistry) GetAllModels(ctx context.Context) (map[string][]*domain.ModelInfo, error) {
	return nil, nil
}
func (m *mockStatusModelRegistry) RemoveEndpoint(ctx context.Context, endpointURL string) error {
