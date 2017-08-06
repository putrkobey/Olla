package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

type mockHTTPClient struct {
	statusCode int
	shouldErr  bool
	delay      time.Duration
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if m.shouldErr {
		return nil, &mockNetError{timeout: false}
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Body:       http.NoBody,
	}, nil
}

type mockNetError struct {
	timeout bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return false }

type mockRepository struct {
	endpoints map[string]*domain.Endpoint
	mu        sync.RWMutex
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		endpoints: make(map[string]*domain.Endpoint),
	}
}

func (m *mockRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	endpoints := make([]*domain.Endpoint, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		endpoints = append(endpoints, ep)
	}
	return endpoints, nil
}

func (m *mockRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthy := make([]*domain.Endpoint, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		if ep.Status == domain.StatusHealthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy, nil
}

func (m *mockRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	routable := make([]*domain.Endpoint, 0, len(m.endpoints))
	for _, ep := range m.endpoints {
		if ep.Status.IsRoutable() {
			routable = append(routable, ep)
		}
	}
	return routable, nil
}

func (m *mockRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := endpoint.URL.String()
	m.endpoints[key] = endpoint
	return nil
}

func (m *mockRepository) LoadFromConfig(ctx context.Context, configs []config.EndpointConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.endpoints = make(map[string]*domain.Endpoint)
	for _, cfg := range configs {
		endpointURL, _ := url.Parse(cfg.URL)
		healthURL, _ := url.Parse(cfg.HealthCheckURL)

		endpoint := &domain.Endpoint{
			Name:                 cfg.Name,
			URL:                  endpointURL,
			HealthCheckURL:       healthURL,
			Status:               domain.StatusUnknown,
			CheckTimeout:         cfg.CheckTimeout,
			URLString:            endpointURL.String(),
			HealthCheckURLString: healthURL.String(),
		}
		m.endpoints[endpointURL.String()] = endpoint
	}
	return nil
}

func (m *mockRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.endpoints[endpointURL.String()]
	return exists
}

func TestHTTPHealthChecker_Check_Success(t *testing.T) {
	mockClient := &mockHTTPClient{statusCode: 200}
	mockRepo := newMockRepository()

	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, cleanup, _ := logger.New(loggerCfg)
	defer cleanup()
	styledLogger := logger.NewPlainStyledLogger(log)

	checker := NewHTTPHealthChecker(mockRepo, styledLogger, mockClient)

	testURL, _ := url.Parse("http://localhost:11434")
	healthURL, _ := url.Parse("/health")
	endpoint := &domain.Endpoint{
		URL:            testURL,
		HealthCheckURL: healthURL,
		CheckTimeout:   time.Second,
	}

	result, err := checker.Check(context.Background(), endpoint)

	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if result.Status != domain.StatusHealthy {
		t.Errorf("Expected StatusHealthy, got %v", result.Status)
	}
}

func TestHTTPHealthChecker_Check_NetworkError(t *testing.T) {
	mockClient := &mockHTTPClient{shouldErr: true}
	mockRepo := newMockRepository()

	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, cleanup, _ := logger.New(loggerCfg)
	defer cleanup()
	styledLogger := logger.NewPlainStyledLogger(log)

	checker := NewHTTPHealthChecker(mockRepo, styledLogger, mockClient)

	testURL, _ := url.Parse("http://localhost:11434")
	healthURL, _ := url.Parse("/health")
	endpoint := &domain.Endpoint{
		URL:            testURL,
		HealthCheckURL: healthURL,
		CheckTimeout:   time.Second,
	}

	result, err := checker.Check(context.Background(), endpoint)

