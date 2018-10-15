package unifier

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// CatalogStore provides thread-safe storage with fast lookups
// by ID, name, and digest. Uses atomic pointers for zero-copy reads.
type CatalogStore struct {
	lastCleanup     time.Time
	catalog         map[string]*atomic.Pointer[domain.UnifiedModel]
	digestIndex     map[string][]string
	nameIndex       map[string][]string
	endpointModels  map[string][]string
	cleanupInterval time.Duration
	mu              sync.RWMutex
}

func NewCatalogStore(cleanupInterval time.Duration) *CatalogStore {
	return &CatalogStore{
		catalog:         make(map[string]*atomic.Pointer[domain.UnifiedModel]),
		digestIndex:     make(map[string][]string),
		nameIndex:       make(map[string][]string),
		endpointModels:  make(map[string][]string),
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
	}
}

// Reset empties the store in place under its lock. Callers hold the store pointer without
// synchronisation, so replacing the store would race with every concurrent reader.
func (c *CatalogStore) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.catalog = make(map[string]*atomic.Pointer[domain.UnifiedModel])
	c.digestIndex = make(map[string][]string)
	c.nameIndex = make(map[string][]string)
	c.endpointModels = make(map[string][]string)
	c.lastCleanup = time.Now()
}

func (c *CatalogStore) GetModel(id string) (*domain.UnifiedModel, bool) {
	c.mu.RLock()
	ptr := c.catalog[id]
	c.mu.RUnlock()

	if ptr == nil {
		return nil, false
	}
	// Zero-copy read via atomic load. The stored model is immutable: it is
	// shared with lock-free readers, so callers must never write to it (or to
	// its slices and maps). Writers call Clone, mutate the clone and PutModel it.
	return ptr.Load(), true
}

func (c *CatalogStore) PutModel(model *domain.UnifiedModel) {
	if model == nil {
		return
	}

	// Copy-on-write: deep copy before storing
	// this allows callers to modify their reference while we store immutable version
	c.putOwned(model.Clone())
}

// putOwned stores a model the caller has already cloned and will not touch again,
// saving the second copy PutModel makes for writers that cloned to mutate.
func (c *CatalogStore) putOwned(modelCopy *domain.UnifiedModel) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ptr, exists := c.catalog[modelCopy.ID]
	if !exists {
		ptr = &atomic.Pointer[domain.UnifiedModel]{}
		c.catalog[modelCopy.ID] = ptr
	}
	ptr.Store(modelCopy)

	if modelCopy.Metadata != nil {
		if digest, ok := modelCopy.Metadata["digest"].(string); ok && digest != "" {
			c.addToIndex(c.digestIndex, digest, modelCopy.ID)
