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

	key := endpointURL.String()
	_, exists := r.endpoints[key]
	return exists
}

func (r *StaticEndpointRepository) LoadFromConfig(ctx context.Context, configs []config.EndpointConfig) error {
	if len(configs) == 0 {
		r.mu.Lock()
		r.endpoints = make(map[string]*domain.Endpoint)
		r.mu.Unlock()
		return nil
	}

	now := time.Now()
	newEndpoints := make(map[string]*domain.Endpoint, len(configs))

	for _, cfg := range configs {
		applyEndpointDefaults(&cfg)
		if err := r.validateEndpointConfig(cfg); err != nil {
			return fmt.Errorf("invalid endpoint config for %q: %w", cfg.Name, err)
		}

		var resolved resolvedAuth
		if cfg.Auth != nil {
			if err := validateAuth(cfg.Name, cfg.Auth); err != nil {
				return err
			}
			var rerr error
			resolved, rerr = resolveAuth(cfg.Name, cfg.Auth)
			if rerr != nil {
				return rerr
			}
		}

		endpointURL, err := url.Parse(cfg.URL)
		if err != nil {
			return fmt.Errorf("invalid endpoint URL %q: %w", cfg.URL, err)
		}

		urlString := endpointURL.String()

		// Resolve URL defaults using fallback hierarchy: explicit config → profile defaults → universal defaults
		healthCheckPath, modelPath := r.resolveURLDefaults(cfg)

		// Build health check and model URLs using ResolveURLPath to preserve
		// the base URL's path prefix. This handles both relative paths and absolute URLs correctly,
		// preserving nested paths like http://localhost:12434/engines/llama.cpp/
		healthCheckURLString := util.ResolveURLPath(urlString, healthCheckPath)
		modelURLString := util.ResolveURLPath(urlString, modelPath)

		healthCheckURL, err := url.Parse(healthCheckURLString)
		if err != nil {
			return fmt.Errorf("invalid health check URL %q: %w", healthCheckURLString, err)
		}

		modelURL, err := url.Parse(modelURLString)
		if err != nil {
			return fmt.Errorf("invalid model URL %q: %w", modelURLString, err)
		}

		newEndpoint := &domain.Endpoint{
			Name:                  cfg.Name,
			URL:                   endpointURL,
			Type:                  cfg.Type,
			Priority:              *cfg.Priority,
			HealthCheckURL:        healthCheckURL,
			ModelUrl:              modelURL,
			ModelFilter:           cfg.ModelFilter,
			CheckInterval:         cfg.CheckInterval,
			CheckTimeout:          cfg.CheckTimeout,
			Status:                domain.StatusUnknown,
			URLString:             urlString,
			HealthCheckPathString: healthCheckPath,
			HealthCheckURLString:  healthCheckURLString,
			ModelURLString:        modelURLString,
			BackoffMultiplier:     1,
			NextCheckTime:         now,
			PreservePath:          cfg.PreservePath,
		}

		if cfg.Auth != nil {
			newEndpoint.AuthHeaderName, newEndpoint.AuthHeaderValue = precomputeAuthHeaders(resolved)
		}

		if len(cfg.Headers) > 0 {
			resolvedHeaders := make(map[string]string, len(cfg.Headers))
			for k, v := range cfg.Headers {
				expanded, herr := envresolver.ExpandStrict(v)
				if herr != nil {
					return fmt.Errorf("endpoint %q: header %q: %w", cfg.Name, k, herr)
				}
				resolvedHeaders[k] = expanded
			}
			newEndpoint.Headers = resolvedHeaders
		}

		newEndpoints[urlString] = newEndpoint
	}

	r.mu.Lock()
	r.endpoints = newEndpoints
	r.mu.Unlock()

	return nil
}

// resolveURLDefaults determines health check and model paths using fallback hierarchy:
// explicit config > profile defaults > universal defaults ("/", "/v1/models")
func (r *StaticEndpointRepository) resolveURLDefaults(cfg config.EndpointConfig) (healthCheckPath, modelPath string) {
	healthCheckPath = cfg.HealthCheckURL
	modelPath = cfg.ModelURL

	if healthCheckPath == "" || modelPath == "" {
		profileHealthPath, profileModelPath := r.applyProfileDefaults(cfg.Type, healthCheckPath, modelPath)
		if healthCheckPath == "" {
			healthCheckPath = profileHealthPath
		}
		if modelPath == "" {
			modelPath = profileModelPath
		}
	}

	return healthCheckPath, modelPath
}

