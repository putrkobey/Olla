package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// testDiscoveryService for testing
type testDiscoveryService struct {
	updatedEndpoint *domain.Endpoint
}

func (t *testDiscoveryService) GetEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (t *testDiscoveryService) GetHealthyEndpoints(ctx context.Context) ([]*domain.Endpoint, error) {
	return nil, nil
}

func (t *testDiscoveryService) RefreshEndpoints(ctx context.Context) error {
	return nil
}

func (t *testDiscoveryService) UpdateEndpointStatus(ctx context.Context, endpoint *domain.Endpoint) error {
	// Capture the endpoint for verification
	t.updatedEndpoint = &domain.Endpoint{}
	*t.updatedEndpoint = *endpoint
	return nil
}

func TestMarkEndpointUnhealthyBackoffProgression(t *testing.T) {
	tests := []struct {
		name                       string
		initialBackoffMultiplier   int
		initialConsecutiveFailures int
		expectedBackoffMultiplier  int
		expectedBackoffInterval    time.Duration
	}{
		{
			name:                       "first_failure",
			initialBackoffMultiplier:   0, // uninitialized
			initialConsecutiveFailures: 0,
			expectedBackoffMultiplier:  2,
			expectedBackoffInterval:    5 * time.Second, // First failure uses normal interval
		},
		{
			name:                       "second_failure",
			initialBackoffMultiplier:   2,
			initialConsecutiveFailures: 1,
			expectedBackoffMultiplier:  4,
			expectedBackoffInterval:    10 * time.Second, // 5s * 2
		},
		{
			name:                       "third_failure",
			initialBackoffMultiplier:   4,
			initialConsecutiveFailures: 2,
			expectedBackoffMultiplier:  8,
			expectedBackoffInterval:    20 * time.Second, // 5s * 4
		},
		{
			name:                       "fourth_failure_capped",
			initialBackoffMultiplier:   8,
			initialConsecutiveFailures: 3,
			expectedBackoffMultiplier:  12,               // capped at 12
			expectedBackoffInterval:    40 * time.Second, // 5s * 8
