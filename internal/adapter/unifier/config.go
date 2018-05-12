package unifier

import (
	"time"
)

type Config struct {
	CircuitBreaker               CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
	DiscoveryRetryPolicy         RetryPolicy          `json:"discovery_retry_policy" yaml:"discovery_retry_policy"`
	ModelTTL                     time.Duration        `json:"model_ttl" yaml:"model_ttl"`
	CleanupInterval              time.Duration        `json:"cleanup_interval" yaml:"cleanup_interval"`
	EndpointHealthCheckInterval  time.Duration        `json:"endpoint_health_check_interval" yaml:"endpoint_health_check_interval"`
	MaxConsecutiveFailures       int                  `json:"max_consecutive_failures" yaml:"max_consecutive_failures"`
	EnableBackgroundCleanup      bool                 `json:"enable_background_cleanup" yaml:"enable_background_cleanup"`
	EnableStateTransitionLogging bool                 `json:"enable_state_transition_logging" yaml:"enable_state_transition_logging"`
}

type RetryPolicy struct {
	MaxAttempts       int           `json:"max_attempts" yaml:"max_attempts"`
	InitialBackoff    time.Duration `json:"initial_backoff" yaml:"initial_backoff"`
