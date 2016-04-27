package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// statusBackend returns a fixed HTTP status code for every request.
func statusBackend(t *testing.T, code int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func probeEndpoint(t *testing.T, rawURL string) *domain.Endpoint {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &domain.Endpoint{
		Name:                 "classify-test",
		URL:                  u,
		HealthCheckURL:       u,
		URLString:            u.String(),
		HealthCheckURLString: u.String(),
		CheckTimeout:         2 * time.Second,
	}
}

func TestDetermineStatus_401_IsConfigError(t *testing.T) {
	t.Parallel()

	srv := statusBackend(t, http.StatusUnauthorized)
	ep := probeEndpoint(t, srv.URL)

	hc := NewHealthClient(http.DefaultClient, NewCircuitBreaker())
	result, err := hc.Check(context.Background(), ep)

	require.NoError(t, err, "401 is a valid HTTP response, not a transport error")
	assert.Equal(t, domain.StatusConfigError, result.Status)
	assert.Equal(t, http.StatusUnauthorized, result.StatusCode)
}

func TestDetermineStatus_403_IsConfigError(t *testing.T) {
	t.Parallel()

	srv := statusBackend(t, http.StatusForbidden)
	ep := probeEndpoint(t, srv.URL)

	hc := NewHealthClient(http.DefaultClient, NewCircuitBreaker())
	result, err := hc.Check(context.Background(), ep)

