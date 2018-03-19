package proxy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// countingEndpointSelector tracks the number of Increment and Decrement calls
// using atomic counters so the test is safe under concurrent execution.
type countingEndpointSelector struct {
	incrementCalls atomic.Int64
	decrementCalls atomic.Int64
	endpoint       *domain.Endpoint
}

func (c *countingEndpointSelector) Select(_ context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	if c.endpoint != nil {
		return c.endpoint, nil
	}
	if len(endpoints) > 0 {
		return endpoints[0], nil
	}
	return nil, nil
}

func (c *countingEndpointSelector) Name() string { return "counting" }

func (c *countingEndpointSelector) IncrementConnections(_ *domain.Endpoint) {
	c.incrementCalls.Add(1)
}

func (c *countingEndpointSelector) DecrementConnections(_ *domain.Endpoint) {
	c.decrementCalls.Add(1)
}

// TestOllaProxy_ConnectionCountingNoDuplication verifies that a single successful proxy
// attempt results in exactly one IncrementConnections call and one DecrementConnections
// call. Before the fix, proxyToSingleEndpoint also incremented/decremented, producing
// counts of two each.
func TestOllaProxy_ConnectionCountingNoDuplication(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
