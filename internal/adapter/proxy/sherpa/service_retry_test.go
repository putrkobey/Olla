package sherpa

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// Simple test implementation of DiscoveryServiceWithEndpointUpdate
type testDiscoveryService struct {
	endpoints         []*domain.Endpoint
	updateStatusCalls []string
}

func (t *testDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return t.endpoints, nil
}

func (t *testDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return t.endpoints, nil
}

func (t *testDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	return nil
}

func (t *testDiscoveryService) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	t.updateStatusCalls = append(t.updateStatusCalls, endpoint.Name)
	return nil
}

// Simple test implementation of EndpointSelector
type testSelector struct {
	selectCount int
	selectIndex int
	endpoints   []*domain.Endpoint
}

func (t *testSelector) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints")
	}

	// For testing retry logic, return endpoints in order
	if t.selectIndex < len(endpoints) {
		ep := endpoints[t.selectIndex]
		t.selectIndex++
		t.selectCount++
		return ep, nil
	}
	return endpoints[0], nil
}

func (t *testSelector) Name() string {
	return "test"
}

func (t *testSelector) IncrementConnections(endpoint *domain.Endpoint) {}
func (t *testSelector) DecrementConnections(endpoint *domain.Endpoint) {}

func createRetryTestLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}

func TestRetryOnConnectionFailure(t *testing.T) {
	mockLogger := createRetryTestLogger()

	// Create test configuration
	config := &Configuration{}
	config.ProxyPrefix = "/olla/"
	config.ConnectionTimeout = time.Second
	config.ConnectionKeepAlive = time.Second
	config.ResponseTimeout = time.Second
	config.ReadTimeout = time.Second
	config.StreamBufferSize = 8192

	// Create test endpoints - first will fail with connection error, second will succeed
	failingEndpoint := &domain.Endpoint{
		Name:   "failing",
		URL:    &url.URL{Scheme: "http", Host: "localhost:9999"}, // Non-existent port
