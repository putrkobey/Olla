package health

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Local constants aliasing the shared backoff defaults
const (
	maxBackoffSeconds    = constants.DefaultMaxBackoffSeconds
	maxBackoffMultiplier = constants.DefaultMaxBackoffMultiplier
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name               string
		endpoint           *domain.Endpoint
		success            bool
		expectedInterval   time.Duration
		expectedMultiplier int
		description        string
	}{
		{
			name: "success_resets_backoff",
			endpoint: &domain.Endpoint{
				CheckInterval:     5 * time.Second,
				BackoffMultiplier: 8,
			},
			success:            true,
			expectedInterval:   5 * time.Second,
			expectedMultiplier: 1,
			description:        "Successful check should reset backoff to normal interval",
		},
		{
			name: "first_failure_keeps_normal_interval",
			endpoint: &domain.Endpoint{
				CheckInterval:     5 * time.Second,
				BackoffMultiplier: 1,
			},
			success:            false,
			expectedInterval:   5 * time.Second, // First failure uses normal interval
			expectedMultiplier: 2,
			description:        "First failure should keep normal interval but set multiplier to 2",
		},
		{
			name: "second_failure_doubles_interval",
			endpoint: &domain.Endpoint{
				CheckInterval:     5 * time.Second,
				BackoffMultiplier: 2,
			},
			success:            false,
			expectedInterval:   10 * time.Second, // Uses current multiplier (2)
			expectedMultiplier: 4,
			description:        "Second failure should double the interval (using multiplier 2)",
		},
		{
			name: "third_failure_exponential_growth",
			endpoint: &domain.Endpoint{
				CheckInterval:     5 * time.Second,
				BackoffMultiplier: 4,
			},
			success:            false,
			expectedInterval:   20 * time.Second, // Uses current multiplier (4)
			expectedMultiplier: 8,
			description:        "Third failure uses multiplier 4 (20s interval)",
		},
		{
			name: "backoff_capped_at_max_multiplier",
			endpoint: &domain.Endpoint{
				CheckInterval:     5 * time.Second,
				BackoffMultiplier: 8,
			},
			success:            false,
			expectedInterval:   40 * time.Second,     // Uses current multiplier (8)
			expectedMultiplier: maxBackoffMultiplier, // Next multiplier capped at max
			description:        "Fourth failure uses multiplier 8, next capped at max",
		},
		{
			name: "backoff_capped_at_max_seconds",
			endpoint: &domain.Endpoint{
				CheckInterval:     10 * time.Second,
				BackoffMultiplier: 6,
			},
			success:            false,
			expectedInterval:   maxBackoffSeconds, // 10s * 6 = 60s
			expectedMultiplier: maxBackoffMultiplier,
