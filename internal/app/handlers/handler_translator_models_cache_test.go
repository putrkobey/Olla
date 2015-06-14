package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestAnthropicModelsCache_HitReturnsSameContent verifies that a cache hit
// returns byte-identical content to the first (miss) response.
func TestAnthropicModelsCache_HitReturnsSameContent(t *testing.T) {
	t.Parallel()

	models := []*domain.UnifiedModel{
		{
			ID:              "claude/3:opus",
			Aliases:         []domain.AliasEntry{{Name: "claude-3-opus", Source: "test"}},
			SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
		},
		{
			ID:              "claude/3:sonnet",
			Aliases:         []domain.AliasEntry{{Name: "claude-3-sonnet", Source: "test"}},
			SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
		},
	}

	app := &Application{
		logger:           &mockStyledLogger{},
		modelRegistry:    &mockTranslatorRegistry{models: models},
		discoveryService: &mockDiscoveryService{},
		repository:       &mockEndpointRepository{},
	}

	trans := mustNewAnthropicTranslator(t)
	handler := app.translatorModelsHandler(trans)

	body1 := doGetModels(t, handler)
	body2 := doGetModels(t, handler)

	if string(body1) != string(body2) {
		t.Errorf("cache hit returned different content:\n  miss: %s\n  hit:  %s", body1, body2)
	}

	// Both responses must be valid JSON with the expected envelope.
	validateModelsEnvelope(t, body1)
}

// TestAnthropicModelsCache_InvalidatesOnModelSetChange verifies that when the
// set of healthy models changes, the cache is invalidated and the response is
// rebuilt with the updated data.
func TestAnthropicModelsCache_InvalidatesOnModelSetChange(t *testing.T) {
	t.Parallel()

	initialModels := []*domain.UnifiedModel{
		{
			ID:              "model/a",
			Aliases:         []domain.AliasEntry{{Name: "model-a", Source: "test"}},
			SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
		},
	}

	reg := &mutableTranslatorRegistry{models: initialModels}
	app := &Application{
		logger:           &mockStyledLogger{},
		modelRegistry:    reg,
		discoveryService: &mockDiscoveryService{},
		repository:       &mockEndpointRepository{},
	}

	trans := mustNewAnthropicTranslator(t)
	handler := app.translatorModelsHandler(trans)

	// First call: miss, builds cache.
	body1 := doGetModels(t, handler)
	resp1 := decodeModelsData(t, body1)
	if len(resp1) != 1 {
		t.Fatalf("expected 1 model, got %d", len(resp1))
	}

	// Simulate a discovery cycle adding a new model.
	reg.mu.Lock()
	reg.models = append(reg.models, &domain.UnifiedModel{
		ID:              "model/b",
		Aliases:         []domain.AliasEntry{{Name: "model-b", Source: "test"}},
		SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
	})
	reg.mu.Unlock()

	// Second call: fingerprint changed, must rebuild.
	body2 := doGetModels(t, handler)
	resp2 := decodeModelsData(t, body2)
	if len(resp2) != 2 {
		t.Fatalf("expected 2 models after registry update, got %d", len(resp2))
	}
}

// TestAnthropicModelsCache_ConcurrentSafety exercises concurrent reads and a
// simultaneous registry update. The -race flag enforces there are no data races.
func TestAnthropicModelsCache_ConcurrentSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test in short mode")
	}
	t.Parallel()

	initialModels := []*domain.UnifiedModel{
		{
			ID:              "concurrent/model",
			SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
		},
	}

	reg := &mutableTranslatorRegistry{models: initialModels}
	app := &Application{
		logger:           &mockStyledLogger{},
		modelRegistry:    reg,
		discoveryService: &mockDiscoveryService{},
		repository:       &mockEndpointRepository{},
	}

	trans := mustNewAnthropicTranslator(t)
	handler := app.translatorModelsHandler(trans)

	const readers = 20
	var wg sync.WaitGroup

	for i := range readers {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 50 {
				doGetModels(t, handler)
			}
			_ = n
		}(i)
	}

	// Simulate discovery updates while readers are running.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 5 {
			reg.mu.Lock()
			reg.models = append(reg.models, &domain.UnifiedModel{
				ID:              "extra/model-" + string(rune('a'+i)),
				SourceEndpoints: []domain.SourceEndpoint{{EndpointURL: "http://localhost:8080"}},
			})
			reg.mu.Unlock()
		}
	}()

	wg.Wait()
}

// TestModelSetFingerprint verifies the fingerprint is order-independent and
// changes when models are added or removed.
func TestModelSetFingerprint(t *testing.T) {
	t.Parallel()

	a := []*domain.UnifiedModel{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	b := []*domain.UnifiedModel{{ID: "c"}, {ID: "a"}, {ID: "b"}} // different order
	c := []*domain.UnifiedModel{{ID: "a"}, {ID: "b"}}            // one fewer
