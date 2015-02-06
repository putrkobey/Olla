package registry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/logger"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type MemoryModelRegistry struct {
	endpointModels   *xsync.Map[string, *domain.EndpointModels]
	modelToEndpoints *xsync.Map[string, *xsync.Map[string, struct{}]]
	// ollamaEndpoints holds the URLs of Ollama-type endpoints. Only their models
	// answer to a name without the default tag; see OllamaDefaultTagName.
	ollamaEndpoints *xsync.Map[string, struct{}]
	logger          logger.StyledLogger
	stats           domain.RegistryStats
	mu              sync.RWMutex
}

func NewMemoryModelRegistry(logger logger.StyledLogger) *MemoryModelRegistry {
	logger.Info("Started in-memory model registry")
	return &MemoryModelRegistry{
		endpointModels:   xsync.NewMap[string, *domain.EndpointModels](),
		modelToEndpoints: xsync.NewMap[string, *xsync.Map[string, struct{}]](),
		ollamaEndpoints:  xsync.NewMap[string, struct{}](),
		stats: domain.RegistryStats{
			TotalEndpoints:    0,
			TotalModels:       0,
			ModelsPerEndpoint: make(map[string]int),
			LastUpdated:       time.Now(),
		},
		logger: logger,
	}
}

func (r *MemoryModelRegistry) RegisterModel(ctx context.Context, endpointURL string, model *domain.ModelInfo) error {
	if err := r.validateInputs(endpointURL, model.Name); err != nil {
		return domain.NewModelRegistryError("register_model", endpointURL, model.Name, err)
	}

	select {
	case <-ctx.Done():
		return domain.NewModelRegistryError("register_model", endpointURL, model.Name, ctx.Err())
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	endpointData, _ := r.endpointModels.LoadOrCompute(endpointURL, func() (newValue *domain.EndpointModels, cancel bool) {
		return &domain.EndpointModels{
			EndpointURL: endpointURL,
			Models:      make([]*domain.ModelInfo, 0, 1),
			LastUpdated: time.Now(),
		}, false
	})

	modelExists := false
	for i, existing := range endpointData.Models {
		if existing.Name == model.Name {
			endpointData.Models[i] = &domain.ModelInfo{
				Name:        model.Name,
				Size:        model.Size,
				Type:        model.Type,
				Description: model.Description,
				LastSeen:    model.LastSeen,
				Details:     model.Details,
			}
			modelExists = true
			break
		}
	}

	if !modelExists {
		endpointData.Models = append(endpointData.Models, &domain.ModelInfo{
			Name:        model.Name,
			Size:        model.Size,
			Type:        model.Type,
			Description: model.Description,
			LastSeen:    model.LastSeen,
			Details:     model.Details,
		})
	}

	endpointData.LastUpdated = time.Now()

	endpointSet, _ := r.modelToEndpoints.LoadOrCompute(model.Name, func() (newValue *xsync.Map[string, struct{}], cancel bool) {
		return xsync.NewMap[string, struct{}](), false
	})
	endpointSet.Store(endpointURL, struct{}{})

	r.updateStats()
	return nil
}

func (r *MemoryModelRegistry) RegisterModels(ctx context.Context, endpointURL string, models []*domain.ModelInfo) error {
	if endpointURL == "" {
		return domain.NewModelRegistryError("register_models", endpointURL, "", errors.New("endpoint URL cannot be empty"))
	}

	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return domain.NewModelRegistryError("register_models", endpointURL, "", fmt.Errorf("invalid endpoint URL: %w", err))
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return domain.NewModelRegistryError("register_models", endpointURL, "", errors.New("endpoint URL must have scheme and host"))
	}

	select {
	case <-ctx.Done():
		return domain.NewModelRegistryError("register_models", endpointURL, "", ctx.Err())
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeEndpointFromIndex(endpointURL)

	if len(models) == 0 {
		r.endpointModels.Delete(endpointURL)
		r.updateStats()
		return nil
	}

	modelsCopy := make([]*domain.ModelInfo, 0, len(models))
	for _, model := range models {
		if model == nil {
			continue // Skip nil models
		}
		if model.Name == "" {
			return domain.NewModelRegistryError("register_models", endpointURL, model.Name, errors.New("model name cannot be empty"))
		}

		modelsCopy = append(modelsCopy, &domain.ModelInfo{
			Name:        model.Name,
			Size:        model.Size,
			Type:        model.Type,
			Description: model.Description,
			LastSeen:    model.LastSeen,
			Details:     model.Details,
		})

		endpointSet, _ := r.modelToEndpoints.LoadOrCompute(model.Name, func() (newValue *xsync.Map[string, struct{}], cancel bool) {
			return xsync.NewMap[string, struct{}](), false
		})
		endpointSet.Store(endpointURL, struct{}{})
	}

	r.endpointModels.Store(endpointURL, &domain.EndpointModels{
		EndpointURL: endpointURL,
		Models:      modelsCopy,
		LastUpdated: time.Now(),
	})

	r.updateStats()
	return nil
}

func (r *MemoryModelRegistry) GetModelsForEndpoint(ctx context.Context, endpointURL string) ([]*domain.ModelInfo, error) {
	if endpointURL == "" {
		return nil, domain.NewModelRegistryError("get_models_for_endpoint", endpointURL, "", errors.New("endpoint URL cannot be empty"))
	}

	select {
	case <-ctx.Done():
		return nil, domain.NewModelRegistryError("get_models_for_endpoint", endpointURL, "", ctx.Err())
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	endpointData, ok := r.endpointModels.Load(endpointURL)
	if !ok {
		return []*domain.ModelInfo{}, nil
	}

	models := make([]*domain.ModelInfo, len(endpointData.Models))
	for i, model := range endpointData.Models {
		models[i] = &domain.ModelInfo{
			Name:        model.Name,
			Size:        model.Size,
			Type:        model.Type,
			Description: model.Description,
			LastSeen:    model.LastSeen,
			Details:     model.Details,
		}
	}

	return models, nil
}

func (r *MemoryModelRegistry) GetEndpointsForModel(ctx context.Context, modelName string) ([]string, error) {
	if modelName == "" {
		return nil, domain.NewModelRegistryError("get_endpoints_for_model", "", modelName, errors.New("model name cannot be empty"))
	}

	select {
	case <-ctx.Done():
		return nil, domain.NewModelRegistryError("get_endpoints_for_model", "", modelName, ctx.Err())
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	endpointSet, ok := r.modelToEndpoints.Load(modelName)
	if !ok {
		// An exact match always wins; only a miss tries the Ollama default tag.
		return r.ollamaDefaultTagEndpoints(modelName), nil
	}

	var endpoints []string
	endpointSet.Range(func(endpoint string, _ struct{}) bool {
		endpoints = append(endpoints, endpoint)
		return true
	})

	return endpoints, nil
}

// exactEndpoints is the exact-name lookup alone, without the default-tag fallback, so
// the unified registry can run every exact match (base and catalogue) before falling back.
func (r *MemoryModelRegistry) exactEndpoints(modelName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpointSet, ok := r.modelToEndpoints.Load(modelName)
	if !ok {
		return nil
	}
	var endpoints []string
	endpointSet.Range(func(endpoint string, _ struct{}) bool {
		endpoints = append(endpoints, endpoint)
