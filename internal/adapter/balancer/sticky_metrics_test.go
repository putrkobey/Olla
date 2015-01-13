package balancer

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestStickySessionWrapper_Stats_TracksMetrics verifies that Stats() reports
// non-zero counters after Select activity. Issue #139: the reporter saw
// /internal/stats/sticky returning all zero counters despite sticky being
// enabled and active. This guards against any future regression where the
// underlying ttlcache stops collecting metrics (e.g. an opt-in option being
// added in a future version).
func TestStickySessionWrapper_Stats_TracksMetrics(t *testing.T) {
	t.Parallel()

	w := makeWrapper(t, defaultStickyConfig())
	ep1 := makeEndpoint("ep1", "http://backend1:8080")
	endpoints := []*domain.Endpoint{ep1}

	// First select: miss → insertion + miss-style get (Get returns nil → not a hit).
	ctx1, _ := injectKey(context.Background(), "stats-key:llama3", "session_header")
	_, err := w.Select(ctx1, endpoints)
	require.NoError(t, err)

	// Second select: hit on the same key → Hits++.
	ctx2, _ := injectKey(context.Background(), "stats-key:llama3", "session_header")
	_, err = w.Select(ctx2, endpoints)
	require.NoError(t, err)

	stats := w.Stats()
	assert.True(t, stats.Enabled, "Stats.Enabled must be true for an active wrapper")
	assert.Equal(t, 1, stats.ActiveSessions, "one unique key should produce one active session")
	assert.GreaterOrEqual(t, stats.Insertions, uint64(1), "insertion counter must be tracked by the underlying ttlcache")
	assert.GreaterOrEqual(t, stats.Hits, uint64(1), "hit counter must be tracked by the underlying ttlcache")
}

// TestComputeStickyKey_PrefixHash_PromptFallback verifies that legacy completions
// requests (which use "prompt" instead of "messages") still produce a sticky key
// via the prefix_hash source. Without the fallback, /v1/completions and
// llamaswap-style passthrough requests bypass sticky entirely and the metrics
// counters never advance.
func TestComputeStickyKey_PrefixHash_PromptFallback(t *testing.T) {
	t.Parallel()

	body := []byte(`{"model":"llama3","prompt":"why is the sky blue?","max_tokens":50}`)
	req, _ := http.NewRequest(http.MethodPost, "/", nil)

	cfg := config.StickySessionConfig{
		KeySources:      []string{"prefix_hash"},
		PrefixHashBytes: 512,
	}
	key, source := ComputeStickyKey(req, "llama3", cfg, body)

	assert.Equal(t, "prefix_hash", source, "prompt-only body must still resolve via prefix_hash")
	assert.NotEmpty(t, key, "prompt fallback must produce a non-empty key")
}

// TestComputeStickyKey_PrefixHash_PreferMessagesOverPrompt ensures the prompt
// fallback does not accidentally override the canonical messages path when both
// are present.
func TestComputeStickyKey_PrefixHash_PreferMessagesOverPrompt(t *testing.T) {
	t.Parallel()

	cfg := config.StickySessionConfig{
		KeySources:      []string{"prefix_hash"},
		PrefixHashBytes: 512,
	}

	chatBody := []byte(`{"messages":[{"role":"user","content":"hi"}]}`)
	mixedBody := []byte(`{"messages":[{"role":"user","content":"hi"}],"prompt":"different content"}`)

	keyChat, _ := ComputeStickyKey(&http.Request{}, "m", cfg, chatBody)
	keyMixed, _ := ComputeStickyKey(&http.Request{}, "m", cfg, mixedBody)

	assert.Equal(t, keyChat, keyMixed, "messages must take precedence over prompt when both exist")
}

// TestComputeStickyKey_SessionHeader_EmptyModel verifies that an unidentified
// model (e.g. llamaswap requests where no inspector populates pr.model) still
// produces a usable, distinct key per session ID. Reproduces a slice of
// issue #139 where requests without model identification appeared to bypass
// sticky entirely.
func TestComputeStickyKey_SessionHeader_EmptyModel(t *testing.T) {
	t.Parallel()

	cfg := config.StickySessionConfig{
		KeySources:      []string{"session_header"},
		PrefixHashBytes: 512,
	}

	req1, _ := http.NewRequest(http.MethodPost, "/", nil)
	req1.Header.Set(constants.HeaderXOllaSessionID, "session-A")

	req2, _ := http.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set(constants.HeaderXOllaSessionID, "session-B")
