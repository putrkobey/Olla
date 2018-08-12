package registry

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/unifier"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// LifecycleUnifiedRegistry implements ManagedService for proper lifecycle integration
// with the service manager. It wraps UnifiedMemoryModelRegistry with retry logic and
// endpoint health tracking.
type LifecycleUnifiedRegistry struct {
	logger logger.StyledLogger
	*UnifiedMemoryModelRegistry
	unifierConfig unifier.Config
	isRunning     atomic.Bool
}

func NewLifecycleUnifiedRegistry(logger logger.StyledLogger, config unifier.Config) *LifecycleUnifiedRegistry {
	unifierFactory := unifier.NewFactory(logger)
	modelUnifier := createUnifier(logger, func() (ports.ModelUnifier, error) {
		return unifierFactory.CreateWithConfig(unifier.LifecycleUnifierType, config)
	}, func() ports.ModelUnifier { return unifier.NewLifecycleUnifier(config, logger) })

	// Build through the plain constructor so every field (routing strategy,
	// endpoint sets) is initialised in one place, then swap in the lifecycle unifier.
	base := NewUnifiedMemoryModelRegistry(logger, nil, nil, nil)
	base.unifier = modelUnifier

	return &LifecycleUnifiedRegistry{
		UnifiedMemoryModelRegistry: base,
		unifierConfig:              config,
		logger:                     logger,
	}
}

func (r *LifecycleUnifiedRegistry) Name() string {
	return "lifecycle-unified-registry"
}

func (r *LifecycleUnifiedRegistry) Start(ctx context.Context) error {
	if !r.isRunning.CompareAndSwap(false, true) {
		return errors.New("registry is already running")
	}

	if lifecycleUnifier, ok := r.unifier.(*unifier.LifecycleUnifier); ok {
		if err := lifecycleUnifier.Start(ctx); err != nil {
			r.isRunning.Store(false)
			return fmt.Errorf("failed to start unifier: %w", err)
		}
	}

	r.logger.Info("Lifecycle registry started")
	return nil
}

func (r *LifecycleUnifiedRegistry) Stop(ctx context.Context) error {
	if !r.isRunning.CompareAndSwap(true, false) {
		return nil
	}

	if lifecycleUnifier, ok := r.unifier.(*unifier.LifecycleUnifier); ok {
		if err := lifecycleUnifier.Stop(ctx); err != nil {
			r.logger.Warn("Error stopping unifier", "error", err)
		}
	}

	r.logger.Info("Lifecycle registry stopped")
	return nil
}

func (r *LifecycleUnifiedRegistry) Dependencies() []string {
	return []string{}
}

// RegisterModelsWithEndpoint adds retry logic to handle transient failures
func (r *LifecycleUnifiedRegistry) RegisterModelsWithEndpoint(ctx context.Context, endpoint *domain.Endpoint, models []*domain.ModelInfo) error {
	retryPolicy := r.unifierConfig.DiscoveryRetryPolicy

	err := unifier.Retry(ctx, retryPolicy, func(ctx context.Context) error {
		return r.UnifiedMemoryModelRegistry.RegisterModelsWithEndpoint(ctx, endpoint, models)
	})

	if err != nil {
