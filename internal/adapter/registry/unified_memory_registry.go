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
	if !exists {
		// Create a minimal endpoint object for backward compatibility
		endpoint = &domain.Endpoint{
			URLString: endpointURL,
			Name:      endpointURL, // Use URL as name if not provided
		}
	}

	// Unify all models for this endpoint
	unifiedModels, err := r.unifier.UnifyModels(ctx, models, endpoint)
	if err != nil {
		r.logger.ErrorWithEndpoint(endpoint.Name, "Failed to unify models", err)
		return
	}

	// Group unified models by ID for merging
	modelGroups := make(map[string][]*domain.UnifiedModel)
	for _, unified := range unifiedModels {
		modelGroups[unified.ID] = append(modelGroups[unified.ID], unified)
	}

	// Merge models across endpoints
	for id, group := range modelGroups {
		// Check if we already have this model globally
		existing, exists := r.globalUnified.Load(id)
		if exists {
			// Merge with existing
			group = append(group, existing)
		}

		merged, err := r.unifier.MergeUnifiedModels(ctx, group)
		if err != nil {
			r.logger.Error("Failed to merge unified models", err)
			continue
		}

		r.globalUnified.Store(id, merged)

		// update cached endpoint set for this model
		var endpointURLs []string
		for _, sourceEndpoint := range merged.SourceEndpoints {
			endpointURLs = append(endpointURLs, sourceEndpoint.EndpointURL)
		}

		// we cache under the unified ID
		r.updateEndpointSet(id, endpointURLs)

		// and also cache under all native names and aliases for fast lookup
		for _, sourceEndpoint := range merged.SourceEndpoints {
			r.updateEndpointSet(sourceEndpoint.NativeName, endpointURLs)
		}
		for _, alias := range merged.Aliases {
			r.updateEndpointSet(alias.Name, endpointURLs)
		}
	}

	// r.logger.InfoWithEndpoint(" ", endpointUrl, "models", len(unifiedModels))
}

// updateEndpointSet updates the cached endpoint set for a given model
// trying to avoid repeated list-to-set conversions in GetHealthyEndpointsForModel
func (r *UnifiedMemoryModelRegistry) updateEndpointSet(modelID string, endpoints []string) {
	endpointSet := xsync.NewMap[string, struct{}]()
	for _, url := range endpoints {
		endpointSet.Store(url, struct{}{})
	}
	r.modelEndpointSets.Store(modelID, endpointSet)
}

// GetEndpointSet returns the cached endpoint set for a given model
func (r *UnifiedMemoryModelRegistry) GetEndpointSet(modelID string) (*xsync.Map[string, struct{}], bool) {
	return r.modelEndpointSets.Load(modelID)
}

// GetUnifiedModels returns all unified models
func (r *UnifiedMemoryModelRegistry) GetUnifiedModels(ctx context.Context) ([]*domain.UnifiedModel, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var models []*domain.UnifiedModel
	r.globalUnified.Range(func(id string, model *domain.UnifiedModel) bool {
		models = append(models, model)
		return true
	})

	return models, nil
}

// GetUnifiedModel returns a specific unified model by ID or alias
func (r *UnifiedMemoryModelRegistry) GetUnifiedModel(ctx context.Context, idOrAlias string) (*domain.UnifiedModel, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Try direct ID lookup first
	if model, exists := r.globalUnified.Load(idOrAlias); exists {
		return model, nil
	}

	// Try alias resolution
	unified, err := r.unifier.ResolveAlias(ctx, idOrAlias)
	if err != nil {
		return nil, fmt.Errorf("model not found: %s", idOrAlias)
	}

	// globalUnified is where RemoveEndpoint and MarkEndpointOffline publish
	// their replacements, so prefer it over the unifier's catalogue copy
	if current, exists := r.globalUnified.Load(unified.ID); exists {
		return current, nil
	}

	return unified, nil
}

// IsModelAvailable overrides to check by unified ID or alias
func (r *UnifiedMemoryModelRegistry) IsModelAvailable(ctx context.Context, modelName string) bool {
	// First check the original registry
	if r.MemoryModelRegistry.IsModelAvailable(ctx, modelName) {
		return true
	}

	// Then check unified models
	_, err := r.GetUnifiedModel(ctx, modelName)
	return err == nil
}

// GetEndpointsForModel overrides to support unified model lookup. Every exact match
// (the base index, then the catalogue's case-insensitive one) is tried before the
// Ollama default-tag fallback, so a bare name never shadows a model that really has it.
func (r *UnifiedMemoryModelRegistry) GetEndpointsForModel(ctx context.Context, modelName string) ([]string, error) {
	if modelName == "" {
		return r.MemoryModelRegistry.GetEndpointsForModel(ctx, modelName)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if endpoints := r.MemoryModelRegistry.exactEndpoints(modelName); len(endpoints) > 0 {
		return endpoints, nil
	}

	unified, _ := r.GetUnifiedModel(ctx, modelName)
	if unified != nil && namesModel(unified, modelName) {
		return sourceURLs(unified, nil), nil
	}

	// Only now may a name without a tag mean "<name>:latest". The fallback is one-way
	// on purpose: Ollama always lists models tagged, so a native bare "foo" never comes
	// from Ollama and "foo:latest" needs no toggle to it.
	if tagged, bare := domain.OllamaDefaultTagName(modelName); bare {
		if endpoints := r.MemoryModelRegistry.defaultTagEndpoints(modelName); len(endpoints) > 0 {
			return endpoints, nil
		}
		if unified != nil {
			// The merged model can also be served elsewhere (the catalogue merges names
			// case-insensitively), and those hosts do not know the bare name.
			return sourceURLs(unified, ollamaSource(tagged)), nil
		}
	}
	if unified == nil {
		return []string{}, nil
	}
	return sourceURLs(unified, nil), nil
}

// CanonicalModelName follows the same exact-first resolution as routing and takes the
// native name from the Ollama source that matched, so stats and sticky keys agree
// whatever case or tag the client (or an alias target) used.
