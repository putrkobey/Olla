package registry

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/adapter/registry/routing"
	"github.com/putrkobey/Olla/internal/adapter/unifier"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// UnifiedMemoryModelRegistry extends MemoryModelRegistry with model unification
type UnifiedMemoryModelRegistry struct {
	unifier         ports.ModelUnifier
	routingStrategy ports.ModelRoutingStrategy
	*MemoryModelRegistry
	unifiedModels     *xsync.Map[string, *domain.UnifiedModel]         // Endpoint -> []UnifiedModel
	globalUnified     *xsync.Map[string, *domain.UnifiedModel]         // UnifiedID -> UnifiedModel (merged across endpoints)
	endpoints         *xsync.Map[string, *domain.Endpoint]             // URL -> Endpoint mapping
	modelEndpointSets *xsync.Map[string, *xsync.Map[string, struct{}]] // ModelID -> Set of endpoint URLs (cached for fast lookup)
	unificationMutex  sync.Mutex
	unifyWG           sync.WaitGroup // tracks in-flight unifyModelsAsync goroutines
}

// createUnifier runs a factory call and, if it fails, logs and builds the caller's fallback
// so a constructor that cannot return an error never carries a nil unifier. The fallback is
// supplied by the caller so it keeps the operator's config and, for the lifecycle registry,
// its unifier type. The built-in unifier types are always registered, so this is defensive
// rather than expected.
func createUnifier(log logger.StyledLogger, create func() (ports.ModelUnifier, error),
	fallback func() ports.ModelUnifier) ports.ModelUnifier {
	u, err := create()
	if err != nil || u == nil {
		log.Error("Failed to create model unifier, falling back", "error", err)
		return fallback()
	}
	return u
}

// NewUnifiedMemoryModelRegistry creates a new registry with unification support
func NewUnifiedMemoryModelRegistry(logger logger.StyledLogger, unificationConfig *config.UnificationConfig,
	routingConfig *config.ModelRoutingStrategy, discovery DiscoveryService) *UnifiedMemoryModelRegistry {
	// Create unifier with config
	unifierFactory := unifier.NewFactory(logger)
	var modelUnifier ports.ModelUnifier

	if unificationConfig != nil && (unificationConfig.StaleThreshold > 0 || unificationConfig.CleanupInterval > 0) {
		// Override only the fields the operator actually set, leaving the rest at
		// the unifier defaults. Each override is guarded so an unset field can't
		// zero out a default (e.g. a cleanup_interval-only config must not blank ModelTTL).
		unifierConfig := unifier.DefaultConfig()
		if unificationConfig.StaleThreshold > 0 {
			unifierConfig.ModelTTL = unificationConfig.StaleThreshold
		}
		if unificationConfig.CleanupInterval > 0 {
			unifierConfig.CleanupInterval = unificationConfig.CleanupInterval
		}
		modelUnifier = createUnifier(logger, func() (ports.ModelUnifier, error) {
			return unifierFactory.CreateWithConfig(unifier.DefaultUnifierType, unifierConfig)
		}, func() ports.ModelUnifier { return unifier.NewDefaultUnifierWithConfig(unifierConfig) })
	} else {
		// Use default config
		modelUnifier = createUnifier(logger, func() (ports.ModelUnifier, error) {
			return unifierFactory.Create(unifier.DefaultUnifierType)
		}, unifier.NewDefaultUnifier)
	}

	// create routing strategy
	var routingStrategy ports.ModelRoutingStrategy
	if routingConfig != nil {
		factory := routing.NewFactory(logger)
		// adapt discovery interface if provided
		var discoveryAdapter ports.DiscoveryService
		if discovery != nil {
			discoveryAdapter = &discoveryServiceAdapter{discovery: discovery}
		}

		// Attempt to create the configured strategy
		strategy, err := factory.Create(*routingConfig, discoveryAdapter)
		if err != nil || strategy == nil {
			// Log the error and fall back to strict strategy
			logger.Error("Failed to create routing strategy, falling back to strict",
				"configured_type", routingConfig.Type,
				"error", err)
			routingStrategy = routing.NewStrictStrategy(logger)
		} else {
			routingStrategy = strategy
		}
	} else {
		// default to strict strategy
		routingStrategy = routing.NewStrictStrategy(logger)
	}

	// Ensure routingStrategy is never nil
	if routingStrategy == nil {
		logger.Warn("Routing strategy was nil, using strict strategy as fallback")
		routingStrategy = routing.NewStrictStrategy(logger)
	}

	return &UnifiedMemoryModelRegistry{
		MemoryModelRegistry: NewMemoryModelRegistry(logger),
		unifier:             modelUnifier,
		routingStrategy:     routingStrategy,
		unifiedModels:       xsync.NewMap[string, *domain.UnifiedModel](),
		globalUnified:       xsync.NewMap[string, *domain.UnifiedModel](),
		endpoints:           xsync.NewMap[string, *domain.Endpoint](),
		modelEndpointSets:   xsync.NewMap[string, *xsync.Map[string, struct{}]](),
	}
}

// RegisterEndpoint stores endpoint information for later use
func (r *UnifiedMemoryModelRegistry) RegisterEndpoint(endpoint *domain.Endpoint) {
	if endpoint != nil && endpoint.GetURLString() != "" {
		r.endpoints.Store(endpoint.GetURLString(), endpoint)
		r.noteEndpointType(endpoint)
	}
}

// RegisterModelsWithEndpoint registers models with full endpoint information
func (r *UnifiedMemoryModelRegistry) RegisterModelsWithEndpoint(ctx context.Context, endpoint *domain.Endpoint, models []*domain.ModelInfo) error {
	// Store endpoint
	r.RegisterEndpoint(endpoint)

	// Register models normally
	return r.RegisterModels(ctx, endpoint.GetURLString(), models)
}

// RegisterModels overrides the base method to add unification
func (r *UnifiedMemoryModelRegistry) RegisterModels(ctx context.Context, endpointURL string, models []*domain.ModelInfo) error {
	// First, register models normally
	if err := r.MemoryModelRegistry.RegisterModels(ctx, endpointURL, models); err != nil {
		return err
	}

	// Invalidate any cached endpoint sets for these models since they're being updated
	for _, model := range models {
		if model != nil {
			r.modelEndpointSets.Delete(model.Name)
			r.dropBareNameSet(model.Name)
		}
	}

	// Then unify them
	r.unifyWG.Add(1)
	go func() {
		defer r.unifyWG.Done()
		r.unifyModelsAsync(ctx, endpointURL, models)
	}()

	return nil
}

// WaitForUnification blocks until every unification started by a prior
// RegisterModels call has been published to the unified view. Callers that need
// read-your-writes (tests, shutdown) use it instead of polling.
func (r *UnifiedMemoryModelRegistry) WaitForUnification() {
	r.unifyWG.Wait()
}

// unifyModelsAsync performs model unification in the background
func (r *UnifiedMemoryModelRegistry) unifyModelsAsync(ctx context.Context, endpointURL string, models []*domain.ModelInfo) {
	r.unificationMutex.Lock()
	defer r.unificationMutex.Unlock()

	// Get or create endpoint object
	endpoint, exists := r.endpoints.Load(endpointURL)
