package balancer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// --- test helpers ---

func defaultStickyConfig() config.StickySessionConfig {
	return config.StickySessionConfig{
		Enabled:         true,
		IdleTTLSeconds:  60,
		MaxSessions:     100,
		KeySources:      []string{"session_header", "prefix_hash", "auth_header", "ip"},
		PrefixHashBytes: 512,
	}
}

func makeEndpoint(name, rawURL string) *domain.Endpoint {
	u, _ := url.Parse(rawURL)
	return &domain.Endpoint{
		Name:      name,
		URL:       u,
		URLString: rawURL,
		Status:    domain.StatusHealthy,
	}
}

func makeWrapper(t *testing.T, cfg config.StickySessionConfig) *StickySessionWrapper {
	t.Helper()
	inner := NewRoundRobinSelector(nil)
	// patch IncrementConnections/DecrementConnections to accept nil statsCollector
	w := NewStickySessionWrapper(inner, cfg)
	w.Start()
	t.Cleanup(w.Stop)
	return w
}

// injectKey builds a context carrying the sticky key and an outcome pointer.
func injectKey(parent context.Context, key, source string) (context.Context, *StickyOutcome) {
	outcome := &StickyOutcome{}
	ctx := context.WithValue(parent, constants.ContextStickyKeyKey, key)
	ctx = context.WithValue(ctx, constants.ContextStickyKeySourceKey, source)
	ctx = context.WithValue(ctx, constants.ContextStickyOutcomeKey, outcome)
	return ctx, outcome
}

// --- RoundRobinSelector with nil stats shim ---
// The existing RoundRobinSelector panics on nil statsCollector only inside
// IncrementConnections/DecrementConnections (which call RecordConnection).
// Select itself works fine, so we can use it directly in unit tests where we
// never call those methods.

// --- tests ---

func TestStickySessionWrapper_Miss(t *testing.T) {
	t.Parallel()

	w := makeWrapper(t, defaultStickyConfig())
	ep1 := makeEndpoint("ep1", "http://backend1:8080")
	ep2 := makeEndpoint("ep2", "http://backend2:8080")
	endpoints := []*domain.Endpoint{ep1, ep2}

	ctx, outcome := injectKey(context.Background(), "sess-abc:llama3", "session_header")

	chosen, err := w.Select(ctx, endpoints)
	require.NoError(t, err)
	assert.NotNil(t, chosen)
	assert.Equal(t, "miss", outcome.Result)
	assert.Equal(t, "session_header", outcome.Source)
}

func TestStickySessionWrapper_Hit(t *testing.T) {
	t.Parallel()

	w := makeWrapper(t, defaultStickyConfig())
	ep1 := makeEndpoint("ep1", "http://backend1:8080")
	ep2 := makeEndpoint("ep2", "http://backend2:8080")
	endpoints := []*domain.Endpoint{ep1, ep2}

	const stickyKey = "sess-hit:llama3"

	// First call — should be a miss and pin a backend.
	ctx1, _ := injectKey(context.Background(), stickyKey, "session_header")
	first, err := w.Select(ctx1, endpoints)
	require.NoError(t, err)

	// Second call with same key — should return the same backend (hit).
	ctx2, outcome2 := injectKey(context.Background(), stickyKey, "session_header")
	second, err := w.Select(ctx2, endpoints)
	require.NoError(t, err)

	assert.Equal(t, first.URLString, second.URLString, "second request should be pinned to the same backend")
	assert.Equal(t, "hit", outcome2.Result)
}

func TestStickySessionWrapper_Repin(t *testing.T) {
	t.Parallel()

	w := makeWrapper(t, defaultStickyConfig())
	ep1 := makeEndpoint("ep1", "http://backend1:8080")
	ep2 := makeEndpoint("ep2", "http://backend2:8080")

	const stickyKey = "sess-repin:llama3"

	// Pin to ep1.
	ctx1, _ := injectKey(context.Background(), stickyKey, "session_header")
	first, err := w.Select(ctx1, []*domain.Endpoint{ep1, ep2})
	require.NoError(t, err)

	// Remove pinned backend from routable set — simulate it going offline.
	remaining := []*domain.Endpoint{ep1, ep2}
	for i, ep := range remaining {
		if ep.URLString == first.URLString {
			remaining[i] = remaining[len(remaining)-1]
			remaining = remaining[:len(remaining)-1]
			break
		}
	}

	ctx2, outcome2 := injectKey(context.Background(), stickyKey, "session_header")
	second, err := w.Select(ctx2, remaining)
	require.NoError(t, err)

	assert.NotEqual(t, first.URLString, second.URLString, "repin should select a different backend")
	assert.Equal(t, "repin", outcome2.Result)
}

func TestStickySessionWrapper_NoKey(t *testing.T) {
	t.Parallel()

	w := makeWrapper(t, defaultStickyConfig())
	ep1 := makeEndpoint("ep1", "http://backend1:8080")

	// Empty key — wrapper should pass through transparently.
	ctx := context.Background()
	outcome := &StickyOutcome{}
	ctx = context.WithValue(ctx, constants.ContextStickyOutcomeKey, outcome)

	chosen, err := w.Select(ctx, []*domain.Endpoint{ep1})
	require.NoError(t, err)
	assert.Equal(t, ep1.URLString, chosen.URLString)
	assert.Equal(t, "disabled", outcome.Result)
	assert.Equal(t, "none", outcome.Source)
}

func TestStickySessionWrapper_TTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL test in short mode")
	}
	t.Parallel()

	cfg := defaultStickyConfig()
	cfg.IdleTTLSeconds = 1 // 1 second for a fast test

	w := makeWrapper(t, cfg)
	ep1 := makeEndpoint("ep1", "http://backend1:8080")
	ep2 := makeEndpoint("ep2", "http://backend2:8080")
	endpoints := []*domain.Endpoint{ep1, ep2}

	const stickyKey = "sess-ttl:llama3"

	ctx1, _ := injectKey(context.Background(), stickyKey, "session_header")
	first, err := w.Select(ctx1, endpoints)
	require.NoError(t, err)

	// Confirm pinned.
	ctx2, outcome2 := injectKey(context.Background(), stickyKey, "session_header")
	second, _ := w.Select(ctx2, endpoints)
	assert.Equal(t, first.URLString, second.URLString)
	assert.Equal(t, "hit", outcome2.Result)

	// ttlcache uses sliding TTL (touch-on-hit) — every Get refreshes the
	// entry. Polling with Select would keep the entry alive forever, so we
	// must wait the full TTL without touching the store. Add a small buffer
	// to allow the background janitor goroutine to run the eviction sweep.
