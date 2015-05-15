package balancer

import (
	"context"
	"hash/fnv"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/tidwall/gjson"
)

// StickyOutcome is the domain type; alias it here for callers that import
// this package directly (e.g. handler layer). The canonical definition lives in
// core/domain so that adapter/proxy/core can read it without a cycle.
type StickyOutcome = domain.StickyOutcome

// StickySessionWrapper is a decorator around any EndpointSelector that adds
// KV-cache affinity routing. It remembers which backend last handled a
// conversation (identified by a computed key) and steers subsequent turns
// to the same backend while it remains routable.
//
// TOCTOU on the session store is intentionally acceptable — both racing
// goroutines will select valid backends and the last writer wins, which
// converges quickly in practice.
type StickySessionWrapper struct {
	inner domain.EndpointSelector
	store *ttlcache.Cache[string, string]
	cfg   config.StickySessionConfig
}

// NewStickySessionWrapper wraps inner with sticky session affinity using cfg.
// Call Start() after construction and Stop() on shutdown.
func NewStickySessionWrapper(inner domain.EndpointSelector, cfg config.StickySessionConfig) *StickySessionWrapper {
	idleTTL := time.Duration(cfg.IdleTTLSeconds) * time.Second

	if cfg.IdleTTLSeconds <= 0 {
		// ttlcache treats a zero TTL as no expiration — sessions accumulate until
		// capacity pressure forces eviction. Warn so operators notice the config.
		slog.Warn("sticky sessions TTL is zero — sessions will never expire by TTL")
	}

	store := ttlcache.New[string, string](
		ttlcache.WithTTL[string, string](idleTTL),
		ttlcache.WithCapacity[string, string](cfg.MaxSessions),
	)

	return &StickySessionWrapper{
		inner: inner,
		store: store,
		cfg:   cfg,
	}
}

// Start launches the ttlcache background goroutine that handles TTL expiry and
// capacity-based eviction. Must be called before the wrapper is used for routing.
func (s *StickySessionWrapper) Start() {
	go s.store.Start()
}

// Stop shuts down the ttlcache background goroutine. Safe to call multiple times.
func (s *StickySessionWrapper) Stop() {
	s.store.Stop()
}

// Name returns a descriptive name that composes the inner balancer name.
func (s *StickySessionWrapper) Name() string {
	return "sticky(" + s.inner.Name() + ")"
}

// IncrementConnections delegates to the inner selector.
func (s *StickySessionWrapper) IncrementConnections(endpoint *domain.Endpoint) {
	s.inner.IncrementConnections(endpoint)
}

// DecrementConnections delegates to the inner selector.
func (s *StickySessionWrapper) DecrementConnections(endpoint *domain.Endpoint) {
	s.inner.DecrementConnections(endpoint)
}

// Select routes the request to the pinned backend when the affinity key is present
// and the backend is still routable. On a miss or dead-backend, it delegates to
// the inner selector and pins the newly chosen backend for next time.
func (s *StickySessionWrapper) Select(ctx context.Context, endpoints []*domain.Endpoint) (*domain.Endpoint, error) {
	outcome, _ := ctx.Value(constants.ContextStickyOutcomeKey).(*StickyOutcome)

	key, _ := ctx.Value(constants.ContextStickyKeyKey).(string)
	source, _ := ctx.Value(constants.ContextStickyKeySourceKey).(string)

	if key == "" {
		// No affinity key — pass through transparently.
		if outcome != nil {
			outcome.Result = "disabled"
			outcome.Source = "none"
		}
		return s.inner.Select(ctx, endpoints)
	}

	// ttlcache.Get refreshes the sliding TTL automatically.
	item := s.store.Get(key)
	if item != nil {
		pinnedURL := item.Value()
		for _, ep := range endpoints {
			if ep.Status.IsRoutable() && ep.URLString == pinnedURL {
				// Sticky hit — backend is still alive and serving this model.
				if outcome != nil {
					outcome.Result = "hit"
					outcome.Source = source
				}
				return ep, nil
			}
		}
		// Pinned backend is gone or unhealthy — fall through to repin.
	}

	chosen, err := s.inner.Select(ctx, endpoints)
	if err != nil {
		return nil, err
	}

	// Record the affinity mapping for future turns.
	s.store.Set(key, chosen.URLString, ttlcache.DefaultTTL)

	result := "miss"
	if item != nil {
		// We had a pin but the backend was no longer routable.
		result = "repin"
	}

	if outcome != nil {
		outcome.Result = result
		outcome.Source = source
	}

	return chosen, nil
}

// PurgeDeadEndpoints removes session entries that point to backends no longer
// present in the provided routable set. Callers can invoke this periodically
