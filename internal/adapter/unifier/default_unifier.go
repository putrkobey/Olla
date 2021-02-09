package unifier

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

type Model struct {
	Metadata      map[string]interface{}
	ID            string
	Name          string
	Family        string
	Digest        string
	Format        string
	Size          int64
	ContextWindow int64
}

type DeduplicationStats struct {
	LastUpdated   time.Time
	TotalModels   int
	UnifiedModels int
	DigestMatches int
	NameMatches   int
}

// DefaultUnifier handles model deduplication across multiple endpoints
// using digest and name matching strategies
type DefaultUnifier struct {
	store          *CatalogStore
	extractor      *ModelExtractor
	stats          DeduplicationStats
	staleThreshold time.Duration
	// statsMu guards stats: UnifyModels writes it while GetStats (the status
	// endpoints) reads it from other goroutines.
	statsMu sync.Mutex
}

func NewDefaultUnifier() ports.ModelUnifier {
	return NewDefaultUnifierWithConfig(DefaultConfig())
}

// NewDefaultUnifierWithConfig builds a DefaultUnifier honouring the supplied
// cleanup interval and model TTL. Zero values fall back to the package defaults,
// so a partially-populated config is safe to pass. This is what lets the
// model_registry.unification.cleanup_interval / stale_threshold config actually
// take effect rather than being silently ignored.
func NewDefaultUnifierWithConfig(config Config) ports.ModelUnifier {
	cleanupInterval := config.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	staleThreshold := config.ModelTTL
	if staleThreshold <= 0 {
		staleThreshold = 24 * time.Hour
	}
	return &DefaultUnifier{
		store:          NewCatalogStore(cleanupInterval),
		extractor:      NewModelExtractor(),
		staleThreshold: staleThreshold,
	}
}

func (u *DefaultUnifier) cleanupStaleModels(threshold time.Duration) {
	if threshold <= 0 {
		threshold = u.staleThreshold
	}
	u.store.CleanupStaleModels(threshold)
}

func (u *DefaultUnifier) UnifyModels(ctx context.Context, models []*domain.ModelInfo, endpoint *domain.Endpoint) ([]*domain.UnifiedModel, error) {
	// Empty model lists are valid - they indicate endpoint has no models anymore
	if models == nil {
		models = []*domain.ModelInfo{}
	}

	if u.store.NeedsCleanup() {
		u.store.CleanupStaleModels(u.staleThreshold)
	}

	endpointURL := endpoint.GetURLString()

	// Remove stale models when endpoint updates its model list
	oldModelIDs := u.store.GetEndpointModels(endpointURL)
	for _, modelID := range oldModelIDs {
		u.removeModelFromEndpoint(modelID, endpointURL)
	}

	processedModels := make([]*domain.UnifiedModel, 0, len(models))
	newModelIDs := make([]string, 0, len(models))

	for _, modelInfo := range models {
		if modelInfo == nil {
			continue
		}

		unified := u.processModel(modelInfo, endpoint)
		if unified != nil {
			processedModels = append(processedModels, unified)
			newModelIDs = append(newModelIDs, unified.ID)
		}
	}

	u.store.SetEndpointModels(endpointURL, newModelIDs)
	u.statsMu.Lock()
	u.stats.TotalModels = u.store.Size()
	u.stats.UnifiedModels = len(processedModels)
	u.stats.LastUpdated = time.Now()
	u.statsMu.Unlock()

	// Results are the store's live immutable pointers; callers must not modify them
	result := make([]*domain.UnifiedModel, 0, len(processedModels))
	for _, model := range processedModels {
		if modelCopy, found := u.store.GetModel(model.ID); found {
			result = append(result, modelCopy)
		}
	}

	return result, nil
}

func (u *DefaultUnifier) processModel(modelInfo *domain.ModelInfo, endpoint *domain.Endpoint) *domain.UnifiedModel {
	model := u.convertModelInfoToModel(modelInfo)

	// Digest matching is most reliable - same binary content
	if model.Digest != "" {
		if existingModels, found := u.store.ResolveByDigest(model.Digest); found && len(existingModels) > 0 {
			existing := existingModels[0]
			u.mergeModel(existing, model, endpoint)
			u.statsMu.Lock()
			u.stats.DigestMatches++
			u.statsMu.Unlock()
			updated, _ := u.store.GetModel(existing.ID)
			return updated
		}
	}

	// Fall back to name matching if no digest
	if existing, found := u.store.ResolveExactName(model.Name); found {
		if u.canMergeByName(existing, model) {
			u.mergeModel(existing, model, endpoint)
			u.statsMu.Lock()
			u.stats.NameMatches++
			u.statsMu.Unlock()
			updated, _ := u.store.GetModel(existing.ID)
			return updated
		}
	}

	unified := u.createUnifiedModel(model, endpoint)
	u.store.PutModel(unified)
	return unified
