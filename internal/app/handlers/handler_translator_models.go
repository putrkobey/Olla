package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// processStartTime is captured once at startup so that created_at in the
// Anthropic models response is stable across calls. There is no genuine
// creation timestamp on UnifiedModel (LastSeen is a discovery heartbeat that
// shifts on every health poll), so process start is the best stable proxy.
var processStartTime = time.Now().UTC().Format(time.RFC3339)

// anthropicModelsCache is a one-entry cache for the Anthropic GET /v1/models
// response. The underlying model set changes only on the 30-second discovery
// cycle, so rebuilding it on every HTTP request is pure waste under load.
//
// Invalidation key: the sorted, comma-joined set of healthy model IDs. A change
// in any model ID (add, remove, rename) produces a different fingerprint and
// triggers a rebuild. Endpoint health changes are captured because we filter
// by healthy endpoints before building the fingerprint.
type anthropicModelsCache struct {
	fingerprint string
	body        []byte // pre-encoded JSON ready to write
	mu          sync.RWMutex
}

// get returns the cached body if the fingerprint matches, otherwise nil.
func (c *anthropicModelsCache) get(fingerprint string) []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.fingerprint == fingerprint {
		return c.body
	}
	return nil
}

// set stores a new fingerprint and pre-encoded body.
func (c *anthropicModelsCache) set(fingerprint string, body []byte) {
	c.mu.Lock()
	c.fingerprint = fingerprint
	c.body = body
	c.mu.Unlock()
}

// modelSetFingerprint builds a stable, cheap change-detection key from the
// set of models as they will be emitted to clients.
//
// The response uses the first alias as the emitted id, not UnifiedModel.ID, so
// the fingerprint must include the emitted value. Without this, an alias change
// (e.g. a version suffix update) while the underlying IDs are stable would leave
// stale alias strings in the cache until the next model-set change. Each entry is
// Each entry is "id\x00emittedID" (null-byte separator) so that changes to
// either field invalidate the cache. A colon would collide with IDs that contain
// colons (e.g. "a:b" + alias "c" vs "a" + alias "b:c" both produce "a:b:c").
// Sorting ensures the key is order-independent.
func modelSetFingerprint(models []*domain.UnifiedModel) string {
	if len(models) == 0 {
		return ""
	}
	ids := make([]string, 0, len(models))
	for _, m := range models {
		emitted := m.ID
		if len(m.Aliases) > 0 {
			emitted = m.Aliases[0].Name
		}
		ids = append(ids, m.ID+"\x00"+emitted)
	}
	sort.Strings(ids)
	return strings.Join(ids, ",")
}

// list models in translator format (eg /olla/anthropic/v1/models)
func (a *Application) translatorModelsHandler(trans translator.RequestTranslator) http.HandlerFunc {
	// One cache per translator registration; lives for the lifetime of the handler.
	cache := &anthropicModelsCache{}

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// fetch available models from registry
		var unifiedModels []*domain.UnifiedModel
		var err error

		// check if registry supports getting unified models
		type unifiedModelsGetter interface {
			GetUnifiedModels(ctx context.Context) ([]*domain.UnifiedModel, error)
		}

		if getter, ok := a.modelRegistry.(unifiedModelsGetter); ok {
			unifiedModels, err = getter.GetUnifiedModels(ctx)
			if err != nil {
				a.writeTranslatorModelsError(w, trans, "failed to get unified models")
				return
			}
		} else {
			a.writeTranslatorModelsError(w, trans, "unified models not supported")
			return
