package health

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pterm/pterm"
	"github.com/putrkobey/Olla/internal/adapter/factory"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	DefaultConcurrentChecks        = 5
	DefaultHealthCheckInterval     = 30 * time.Second
	DefaultRecoveryCallbackTimeout = 10 * time.Second
)

type HTTPHealthChecker struct {
	repository        domain.EndpointRepository
	logger            logger.StyledLogger
	recoveryCallback  RecoveryCallback
	unhealthyCallback UnhealthyCallback
	healthClient      *HealthClient
	ticker            *time.Ticker
	stopCh            chan struct{}
	isRunning         atomic.Bool
}

func NewHTTPHealthChecker(repository domain.EndpointRepository, logger logger.StyledLogger, client HTTPClient) *HTTPHealthChecker {
	circuitBreaker := NewCircuitBreaker()
	healthClient := NewHealthClient(client, circuitBreaker)

	return &HTTPHealthChecker{
		healthClient:     healthClient,
		repository:       repository,
		logger:           logger,
		stopCh:           make(chan struct{}),
		recoveryCallback: NoOpRecoveryCallback{},
	}
}

// SetRecoveryCallback sets the callback to be invoked when an endpoint recovers
func (c *HTTPHealthChecker) SetRecoveryCallback(callback RecoveryCallback) {
	if callback != nil {
		c.recoveryCallback = callback
	}
}

// SetUnhealthyCallback sets the callback to be invoked when an endpoint transitions to an unhealthy state.
func (c *HTTPHealthChecker) SetUnhealthyCallback(callback UnhealthyCallback) {
	if callback != nil {
		c.unhealthyCallback = callback
	}
}

func NewHTTPHealthCheckerWithDefaults(repository domain.EndpointRepository, logger logger.StyledLogger) *HTTPHealthChecker {
	// We want to enable connection pooling and reuse with some sane defaults
	// Probes carry the endpoint's auth headers, so the client comes from the shared
	// backend constructor: no environment proxy and no cross-origin redirects.
	// Per-probe timeouts are already enforced by the CheckTimeout context in
	// performSingleCheck; the header timeout is a backstop against backends that
	// accept the TCP connection but then never send a response header.
	client := factory.NewBackendHTTPClient(factory.BackendClientOptions{
		Timeout:               DefaultHealthCheckerTimeout,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       30 * time.Second,
		ResponseHeaderTimeout: DefaultHealthCheckerResponseHeaderTimeout,
	})
	return NewHTTPHealthChecker(repository, logger, client)
}

func (c *HTTPHealthChecker) Check(ctx context.Context, endpoint *domain.Endpoint) (domain.HealthCheckResult, error) {
	// Add timeout protection for individual checks
	checkCtx, cancel := context.WithTimeout(ctx, endpoint.CheckTimeout*2) // Give extra time for retries
	defer cancel()

	return c.healthClient.Check(checkCtx, endpoint)
}

func (c *HTTPHealthChecker) StartChecking(ctx context.Context) error {
	if c.isRunning.Load() {
		return nil
	}

	endpoints, err := c.repository.GetAll(ctx)
	if err != nil {
		return domain.NewEndpointError("start_health_checking", "repository",
			fmt.Errorf("failed to get endpoints for health checking: %w", err))
	}

	c.logger.Info("Starting Health Checker Service", "check_interval", DefaultHealthCheckInterval, "endpoints", len(endpoints))

	c.isRunning.Store(true)

	c.ticker = time.NewTicker(DefaultHealthCheckInterval)
	go c.healthCheckLoop(ctx)

	return nil
}

func (c *HTTPHealthChecker) StopChecking(ctx context.Context) error {
	// CAS from true→false is the single guard; concurrent callers that lose the
	// race see false and return early without touching stopCh.
	if !c.isRunning.CompareAndSwap(true, false) {
		return nil
	}

	if c.ticker != nil {
		c.ticker.Stop()
	}

	close(c.stopCh)

	return nil
}

func (c *HTTPHealthChecker) healthCheckLoop(ctx context.Context) {
	// Function-level safety net: if something truly unrecoverable escapes the
	// per-tick inner recover, mark isRunning false so StartChecking can restart.
	defer func() {
		if c.ticker != nil {
			c.ticker.Stop()
		}
		if r := recover(); r != nil {
			c.logger.Error("Health check loop exited unexpectedly", "panic", r)
			c.isRunning.Store(false)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("Health check loop stopping due to context cancellation")
			return
		case <-c.stopCh:
			c.logger.Debug("Health check loop stopping due to stop signal")
			return
		case <-c.ticker.C:
			// Wrap per-tick work in its own recover so a panic in
			// performHealthChecks does not kill the goroutine - the loop
			// continues and the next tick fires normally.
			func() {
				defer func() {
					if r := recover(); r != nil {
						c.logger.Error("Health check tick panic recovered, loop continues",
							"panic", r)
					}
				}()
				// Derive from the loop's own context so in-flight checks are
				// cancelled when the checker shuts down, rather than running
				// to their full timeout against an already-stopped service.
				checkCtx, cancel := context.WithTimeout(ctx, DefaultHealthCheckInterval/2)
				defer cancel()
				c.performHealthChecks(checkCtx)
			}()
		}
	}
}

func (c *HTTPHealthChecker) performHealthChecks(ctx context.Context) {
	endpoints, err := c.repository.GetAll(ctx)
	if err != nil {
		c.logger.Error("Failed to get endpoints for health checking",
			"error", domain.NewEndpointError("get_endpoints", "repository", err))
		return
	}

	now := time.Now()
	var wg sync.WaitGroup

	endpointsToCheck := make([]*domain.Endpoint, 0, len(endpoints))

	// Filter endpoints that are due for checking.
	// Skip endpoints still inside a rate-limit window; hammering a throttled backend
	// will not make it respond faster and wastes quota.
	for _, endpoint := range endpoints {
