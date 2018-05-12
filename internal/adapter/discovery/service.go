package discovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/putrkobey/Olla/internal/adapter/filter"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	MaxConsecutiveFailures   = 5 // Pause discovery after this many failures
	DefaultDiscoveryCooldown = 5 * time.Minute
	DefaultCleanupInterval   = 10 * time.Minute
)

// ModelDiscoveryService coordinates model discovery across all endpoints
type ModelDiscoveryService struct {
	client            ModelDiscoveryClient
	endpointRepo      domain.EndpointRepository
	modelRegistry     domain.ModelRegistry
	logger            logger.StyledLogger
	modelFilter       ports.Filter
	stopCh            chan struct{}
	ticker            *time.Ticker
	retryAfter        map[string]time.Time
	disabledEndpoints map[string]int                  // tracks consecutive failures
	endpointFilters   map[string]*domain.FilterConfig // per-endpoint filter configs
	config            DiscoveryConfig
	mu                sync.RWMutex
	isRunning         atomic.Bool
}

// DiscoveryConfig holds configuration for the discovery service
type DiscoveryConfig struct {
	Interval          time.Duration // How often to discover models
	Timeout           time.Duration // Timeout for individual discovery requests
	ConcurrentWorkers int           // Max concurrent discovery operations
	RetryAttempts     int           // Number of retry attempts for failed discoveries
	RetryBackoff      time.Duration // Backoff between retries
}

func NewModelDiscoveryService(client ModelDiscoveryClient, endpointRepo domain.EndpointRepository, modelRegistry domain.ModelRegistry, config DiscoveryConfig, logger logger.StyledLogger) *ModelDiscoveryService {
	return &ModelDiscoveryService{
		client:            client,
		endpointRepo:      endpointRepo,
		modelRegistry:     modelRegistry,
		logger:            logger,
		config:            config,
		stopCh:            make(chan struct{}),
		disabledEndpoints: make(map[string]int),
		retryAfter:        make(map[string]time.Time),
		modelFilter:       filter.NewGlobFilter(),
		endpointFilters:   make(map[string]*domain.FilterConfig),
	}
}

func (s *ModelDiscoveryService) Start(ctx context.Context) error {
	if !s.isRunning.CompareAndSwap(false, true) {
		return errors.New("discovery service is already running")
	}

	// Guard against a zero interval from user config overrides — time.NewTicker panics on <= 0.
	if s.config.Interval <= 0 {
		s.logger.Warn("Discovery interval is zero or negative, defaulting to 5 minutes", "configured", s.config.Interval)
		s.config.Interval = 5 * time.Minute
	}

	s.logger.Info("Initialising model discovery service", "interval", s.config.Interval)

	s.ticker = time.NewTicker(s.config.Interval)

	go s.discoveryLoop(ctx)

	return nil
}

func (s *ModelDiscoveryService) Stop(ctx context.Context) error {
	if !s.isRunning.CompareAndSwap(true, false) {
		return nil // looks like this has stopped earlier?
	}

	s.logger.Info("Stopping model discovery service")

	if s.ticker != nil {
		s.ticker.Stop()
	}

	close(s.stopCh)
	return nil
}

// discoveryLoop runs the periodic discovery process
func (s *ModelDiscoveryService) discoveryLoop(ctx context.Context) {
	defer s.isRunning.Store(false)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-s.ticker.C:
			if err := s.DiscoverAll(ctx); err != nil {
				s.LogErrorsWithContext(err)
			}
		}
	}
}
func (s *ModelDiscoveryService) LogErrorsWithContext(err error) {
	userMsg := GetUserFriendlyMessage(err)
	logCtx := logger.LogContext{
		UserArgs:     []interface{}{},
		DetailedArgs: []interface{}{"detailed_error", err},
	}
	s.logger.WarnWithContext(fmt.Sprintf("Regular discovery failed (%s)", userMsg), "", logCtx)
}

// DiscoverAll discovers models from all healthy endpoints
func (s *ModelDiscoveryService) DiscoverAll(ctx context.Context) error {
	endpoints, err := s.endpointRepo.GetHealthy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get healthy endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		s.logger.Debug("No healthy endpoints available for discovery")
		return nil
	}

	// Failed endpoints are retried once their cooldown expires.
	activeEndpoints := s.filterActiveEndpoints(endpoints)
	if len(activeEndpoints) == 0 {
		s.logger.Debug("No active endpoints available for discovery")
		return nil
	}

	s.logger.InfoWithCount("Starting model discovery on healthy endpoints", len(activeEndpoints))

	// Use worker pool for concurrent discovery
	return s.discoverConcurrently(ctx, activeEndpoints)
}

// endpointModelRegistrar is implemented by registries that register models with the
// endpoint object rather than just its URL.
type endpointModelRegistrar interface {
	RegisterModelsWithEndpoint(ctx context.Context, endpoint *domain.Endpoint, models []*domain.ModelInfo) error
}

// DiscoverEndpoint discovers models from a specific endpoint
func (s *ModelDiscoveryService) DiscoverEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	// Explicit discovery bypasses the periodic cooldown.
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}

	models, err := s.fetchModelsWithRetry(ctx, endpoint)
	if err != nil {
		if ctx.Err() == nil && !errors.Is(err, errDiscoveryStopped) {
			s.handleDiscoveryError(endpoint, err)
		}
