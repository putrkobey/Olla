package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

func newPolicyRetryHandler(t *testing.T, ds *testDiscoveryService, p proxyconfig.RetryPolicy) *RetryHandler {
	t.Helper()
	log, _, _ := logger.New(&logger.Config{Level: "error"})
	return NewRetryHandler(ds, logger.NewPlainStyledLogger(log), WithRetryPolicy(p))
}

// TestExecuteWithRetry_Policy drives the handler with a backend set where every
// endpoint but the last refuses the connection, so the attempt count shows
// exactly how much failover the policy allowed.
func TestExecuteWithRetry_Policy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		policy       proxyconfig.RetryPolicy
		method       string
		endpoints    int
		wantAttempts int
