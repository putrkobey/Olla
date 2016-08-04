package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/adapter/discovery"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

type testRecoveryCallback struct {
	mu       sync.Mutex
	called   bool
	endpoint *domain.Endpoint
}

func (t *testRecoveryCallback) OnEndpointRecovered(ctx context.Context, endpoint *domain.Endpoint) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.called = true
	t.endpoint = endpoint
	return nil
}

func (t *testRecoveryCallback) wasCalledWith() (bool, *domain.Endpoint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.called, t.endpoint
}

func TestHealthCheckerRecoveryCallback(t *testing.T) {
	// Create a test server that will be "down" initially then come back up
	serverIsHealthy := false
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serverIsHealthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer testServer.Close()

