package unifier

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type DiscoveryClient interface {
	DiscoverModels(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error)
}

// LifecycleUnifier manages model lifecycle with automatic cleanup,
// health monitoring, and circuit breaker support
type LifecycleUnifier struct {
	unifier         ports.ModelUnifier
	discoveryClient DiscoveryClient
	logger          logger.StyledLogger

	cleanupCtx      context.Context
	endpointManager *EndpointManager

	cleanupCancel context.CancelFunc
	config        Config
	cleanupWg     sync.WaitGroup
	isRunning     atomic.Bool
}

func NewLifecycleUnifier(config Config, logger logger.StyledLogger) ports.ModelUnifier {
	if err := config.Validate(); err != nil {
		config = DefaultConfig()
	}

	return &LifecycleUnifier{
		unifier:         NewDefaultUnifier(),
		endpointManager: NewEndpointManager(config, logger),
		config:          config,
		logger:          logger,
	}
}

func (u *LifecycleUnifier) SetDiscoveryClient(client DiscoveryClient) {
	u.discoveryClient = client
}

func (u *LifecycleUnifier) Start(ctx context.Context) error {
	if !u.isRunning.CompareAndSwap(false, true) {
		return errors.New("unifier is already running")
	}

	u.cleanupCtx, u.cleanupCancel = context.WithCancel(ctx)

	if u.config.EnableBackgroundCleanup {
		u.cleanupWg.Add(1)
		go u.cleanupRoutine()
	}

	u.logger.Info("Lifecycle unifier started",
		"model_ttl", u.config.ModelTTL,
		"cleanup_interval", u.config.CleanupInterval,
		"background_cleanup", u.config.EnableBackgroundCleanup)

	return nil
}

func (u *LifecycleUnifier) Stop(ctx context.Context) error {
	if !u.isRunning.CompareAndSwap(true, false) {
		return nil
	}

	u.logger.Info("Stopping lifecycle unifier")

	if u.cleanupCancel != nil {
		u.cleanupCancel()
	}

	done := make(chan struct{})
	go func() {
		u.cleanupWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		u.logger.Info("Lifecycle unifier stopped gracefully")
	case <-ctx.Done():
		u.logger.Warn("Lifecycle unifier stop timeout")
		return ctx.Err()
	}

	return nil
}

func (u *LifecycleUnifier) UnifyModels(ctx context.Context, models []*domain.ModelInfo, endpoint *domain.Endpoint) ([]*domain.UnifiedModel, error) {
	endpointURL := endpoint.GetURLString()

	// Circuit breaker prevents cascading failures
	if u.config.CircuitBreaker.Enabled {
		cb := u.endpointManager.GetCircuitBreaker(endpointURL)
		if cb != nil && !cb.Allow() {
			return nil, fmt.Errorf("circuit breaker open for endpoint %s", endpoint.Name)
		}
	}

	unified, err := u.unifier.UnifyModels(ctx, models, endpoint)
	if err != nil {
		u.endpointManager.RecordFailure(endpointURL, err)
		return nil, err
	}

	u.endpointManager.RecordSuccess(endpointURL)
	u.updateEndpointStates(unified, endpointURL)

	return unified, nil
}

func (u *LifecycleUnifier) updateEndpointStates(models []*domain.UnifiedModel, endpointURL string) {
	stateInfo := u.endpointManager.GetState(endpointURL)
	if stateInfo == nil {
		return
	}

	// UnifyModels returns the store's live immutable pointers, which lock-free
	// readers may hold. Decorate private clones and swap them into the result
	// rather than writing through the shared pointers.
	for idx, model := range models {
		var clone *domain.UnifiedModel
		for i := range model.SourceEndpoints {
			if model.SourceEndpoints[i].EndpointURL != endpointURL {
				continue
			}
			if clone == nil {
				clone = model.Clone()
			}
			clone.SourceEndpoints[i].StateInfo = stateInfo
			clone.SourceEndpoints[i].LastStateCheck = time.Now()
		}
		if clone != nil {
			models[idx] = clone
			// persist the decoration: later lookups and discovery cycles read the catalogue
			if p, ok := u.unifier.(modelPublisher); ok {
				p.publishModel(clone)
			}
		}
	}
}

// modelPublisher is implemented by unifiers that can swap a replacement record
// into their catalogue; the clone passed in must not be touched afterwards.
type modelPublisher interface {
	publishModel(model *domain.UnifiedModel)
}

func (u *LifecycleUnifier) cleanupRoutine() {
	defer u.cleanupWg.Done()

	ticker := time.NewTicker(u.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-u.cleanupCtx.Done():
			return
		case <-ticker.C:
			u.performCleanup()
		}
	}
}

func (u *LifecycleUnifier) performCleanup() {
	// Trigger model TTL cleanup in the underlying unifier
	if defaultUnifier, ok := u.unifier.(*DefaultUnifier); ok {
		defaultUnifier.cleanupStaleModels(u.config.ModelTTL)
	}
	activeEndpoints := make(map[string]bool)

	if extUnifier, ok := u.unifier.(ExtendedUnifier); ok {
		models, _ := extUnifier.GetAllModels(context.Background())
		for _, model := range models {
			for _, endpoint := range model.SourceEndpoints {
				activeEndpoints[endpoint.EndpointURL] = true
			}
		}
	}

	u.endpointManager.CleanupOrphaned(activeEndpoints)
}

func (u *LifecycleUnifier) UnifyModel(ctx context.Context, sourceModel *domain.ModelInfo, endpoint *domain.Endpoint) (*domain.UnifiedModel, error) {
	models := []*domain.ModelInfo{sourceModel}
	unified, err := u.UnifyModels(ctx, models, endpoint)
	if err != nil {
		return nil, err
	}
	if len(unified) > 0 {
		return unified[0], nil
