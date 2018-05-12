package discovery

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/pkg/envresolver"
)

const (
	MinHealthCheckInterval = 1 * time.Second
	MaxHealthCheckTimeout  = 30 * time.Second

	// Defaults applied when the user omits timing fields in their endpoint config.
	// Without these, a zero value would fail the minimum-interval validator on startup.
	DefaultCheckInterval = 5 * time.Second
	DefaultCheckTimeout  = 2 * time.Second
	DefaultPriority      = 100
)

type StaticEndpointRepository struct {
	endpoints      map[string]*domain.Endpoint
	profileFactory *profile.Factory
	mu             sync.RWMutex
}

func NewStaticEndpointRepository() *StaticEndpointRepository {
	profileFactory, err := profile.NewFactoryWithDefaults()
	if err != nil {
		// For tests, use empty profile dir to get built-in profiles
		profileFactory, _ = profile.NewFactory("")
	}
	return &StaticEndpointRepository{
		endpoints:      make(map[string]*domain.Endpoint),
		profileFactory: profileFactory,
	}
}

func NewStaticEndpointRepositoryWithFactory(factory *profile.Factory) *StaticEndpointRepository {
	return &StaticEndpointRepository{
		endpoints:      make(map[string]*domain.Endpoint),
		profileFactory: factory,
	}
}

func (r *StaticEndpointRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.endpoints) == 0 {
		return []*domain.Endpoint{}, nil
	}

	endpoints := make([]*domain.Endpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		endpointCopy := *endpoint
		endpoints = append(endpoints, &endpointCopy)
	}
	return endpoints, nil
}

func (r *StaticEndpointRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pre-allocate with capacity of all endpoints (worst case)
	healthy := make([]*domain.Endpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		if endpoint.Status == domain.StatusHealthy {
			healthyCopy := *endpoint
			healthy = append(healthy, &healthyCopy)
		}
	}

	return healthy, nil
}

func (r *StaticEndpointRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Pre-allocate with capacity of all endpoints (worst case)
	routable := make([]*domain.Endpoint, 0, len(r.endpoints))
	for _, endpoint := range r.endpoints {
		if endpoint.Status.IsRoutable() {
			routableCopy := *endpoint
			routable = append(routable, &routableCopy)
		}
	}

	return routable, nil
}

func (r *StaticEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := endpoint.URL.String()
	existing, exists := r.endpoints[key]
	if !exists {
		return &domain.EndpointNotFoundError{URL: key}
	}

	existing.Status = endpoint.Status
	existing.LastChecked = endpoint.LastChecked
	existing.ConsecutiveFailures = endpoint.ConsecutiveFailures
	existing.BackoffMultiplier = endpoint.BackoffMultiplier
	existing.NextCheckTime = endpoint.NextCheckTime
	existing.LastLatency = endpoint.LastLatency
	existing.RateLimitedUntil = endpoint.RateLimitedUntil

	return nil
}

func (r *StaticEndpointRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
