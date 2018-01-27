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

// authEnforcingBackend returns 401 when the expected header is absent or wrong,
// and 200 when it matches. This proves auth is actually transported, not just set.
func authEnforcingBackend(t *testing.T, headerName, wantValue string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(headerName) != wantValue {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func makeEndpoint(t *testing.T, rawURL string) *domain.Endpoint {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return &domain.Endpoint{
		Name:                 "test",
		URL:                  u,
		HealthCheckURL:       u,
		URLString:            u.String(),
		HealthCheckURLString: u.String(),
		CheckTimeout:         2 * time.Second,
	}
}

// TestHealthProbe_BearerAuth proves that a probe on an endpoint with bearer auth
// reaches the backend with the correct Authorization header and is classified healthy.
func TestHealthProbe_BearerAuth(t *testing.T) {
	t.Parallel()

	const token = "Bearer secret-token"
	srv := authEnforcingBackend(t, "Authorization", token)

	ep := makeEndpoint(t, srv.URL)
	ep.AuthHeaderName = "Authorization"
	ep.AuthHeaderValue = token

	hc := NewHealthClient(http.DefaultClient, NewCircuitBreaker())
