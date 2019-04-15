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
