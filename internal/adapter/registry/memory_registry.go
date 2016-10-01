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
		return true
	})
	return endpoints
}

func (r *MemoryModelRegistry) defaultTagEndpoints(modelName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ollamaDefaultTagEndpoints(modelName)
}

// RegisterModelsWithEndpoint registers models and remembers the endpoint's type,
// which the base registry otherwise never sees, so the Ollama default-tag rule can
// be scoped to Ollama-sourced models.
func (r *MemoryModelRegistry) RegisterModelsWithEndpoint(ctx context.Context, endpoint *domain.Endpoint, models []*domain.ModelInfo) error {
	r.noteEndpointType(endpoint)
	return r.RegisterModels(ctx, endpoint.GetURLString(), models)
}

func (r *MemoryModelRegistry) noteEndpointType(endpoint *domain.Endpoint) {
	if endpoint == nil || endpoint.GetURLString() == "" {
		return
	}
	if endpoint.Type == domain.ProfileOllama {
		r.ollamaEndpoints.Store(endpoint.GetURLString(), struct{}{})
		return
	}
	r.ollamaEndpoints.Delete(endpoint.GetURLString())
}

// ollamaDefaultTagEndpoints answers a name without a tag from Ollama-sourced
// "name:latest" entries. Callers have already missed the exact lookup, so a model
// that really is called "name" is never shadowed. Scoping to Ollama endpoints
// matters: a vLLM host serving "name:latest" has not been asked for "name" and
// would be sent a name it does not know. Callers hold r.mu.
func (r *MemoryModelRegistry) ollamaDefaultTagEndpoints(modelName string) []string {
	tagged, ok := domain.OllamaDefaultTagName(modelName)
	if !ok {
		return []string{}
	}
	endpointSet, ok := r.modelToEndpoints.Load(tagged)
	if !ok {
		return []string{}
	}
	endpoints := []string{}
	endpointSet.Range(func(endpoint string, _ struct{}) bool {
		if _, isOllama := r.ollamaEndpoints.Load(endpoint); isOllama {
			endpoints = append(endpoints, endpoint)
		}
		return true
	})
	return endpoints
}

// CanonicalModelName returns the native name a request for name resolves to, so
// stats and sticky keys for "foo" and "foo:latest" land together. It differs from
// name only when the exact lookup misses and an Ollama host serves "name:latest".
// Forwarded request bodies are never rewritten with it.
func (r *MemoryModelRegistry) CanonicalModelName(name string) string {
	tagged, ok := domain.OllamaDefaultTagName(name)
	if !ok {
		return name
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exact := r.modelToEndpoints.Load(name); exact {
		return name
	}
	if len(r.ollamaDefaultTagEndpoints(name)) > 0 {
		return tagged
	}
	return name
}

func (r *MemoryModelRegistry) IsModelAvailable(ctx context.Context, modelName string) bool {
	if modelName == "" {
		return false
	}

	select {
	case <-ctx.Done():
		return false
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	endpointSet, ok := r.modelToEndpoints.Load(modelName)
	if !ok {
		return len(r.ollamaDefaultTagEndpoints(modelName)) > 0
	}

	// Check if the set has any endpoints
	hasEndpoints := false
	endpointSet.Range(func(_ string, _ struct{}) bool {
		hasEndpoints = true
		return false // Stop after first item
	})

	return hasEndpoints
}

func (r *MemoryModelRegistry) GetAllModels(ctx context.Context) (map[string][]*domain.ModelInfo, error) {
	select {
	case <-ctx.Done():
		return nil, domain.NewModelRegistryError("get_all_models", "", "", ctx.Err())
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*domain.ModelInfo)
	r.endpointModels.Range(func(endpointURL string, endpointData *domain.EndpointModels) bool {
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
		result[endpointURL] = models
		return true
	})

	return result, nil
}

func (r *MemoryModelRegistry) GetEndpointModelMap(ctx context.Context) (map[string]*domain.EndpointModels, error) {
	select {
	case <-ctx.Done():
		return nil, domain.NewModelRegistryError("get_endpoint_model_map", "", "", ctx.Err())
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*domain.EndpointModels)
	r.endpointModels.Range(func(endpointURL string, endpointData *domain.EndpointModels) bool {
		result[endpointURL] = &domain.EndpointModels{
			EndpointURL: endpointData.EndpointURL,
			Models:      endpointData.Models,
			LastUpdated: endpointData.LastUpdated,
		}
		return true
	})

	return result, nil
}

func (r *MemoryModelRegistry) RemoveEndpoint(ctx context.Context, endpointURL string) error {
	if endpointURL == "" {
		return domain.NewModelRegistryError("remove_endpoint", endpointURL, "", errors.New("endpoint URL cannot be empty"))
	}

	select {
	case <-ctx.Done():
		return domain.NewModelRegistryError("remove_endpoint", endpointURL, "", ctx.Err())
	default:
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.removeEndpointFromIndex(endpointURL)
	r.endpointModels.Delete(endpointURL)
	r.ollamaEndpoints.Delete(endpointURL)
	r.updateStats()

	return nil
}

func (r *MemoryModelRegistry) GetStats(ctx context.Context) (domain.RegistryStats, error) {
	select {
	case <-ctx.Done():
		return domain.RegistryStats{}, domain.NewModelRegistryError("get_stats", "", "", ctx.Err())
	default:
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	modelsPerEndpoint := make(map[string]int, len(r.stats.ModelsPerEndpoint))
	for k, v := range r.stats.ModelsPerEndpoint {
		modelsPerEndpoint[k] = v
	}

	return domain.RegistryStats{
		TotalEndpoints:    r.stats.TotalEndpoints,
		TotalModels:       r.stats.TotalModels,
		ModelsPerEndpoint: modelsPerEndpoint,
		LastUpdated:       r.stats.LastUpdated,
	}, nil
}

func (r *MemoryModelRegistry) validateInputs(endpointURL, modelName string) error {
	if endpointURL == "" {
		return errors.New("endpoint URL cannot be empty")
	}

	if modelName == "" {
		return errors.New("model name cannot be empty")
	}

	if strings.TrimSpace(endpointURL) == "" {
		return errors.New("endpoint URL cannot be whitespace only")
	}

	if strings.TrimSpace(modelName) == "" {
		return errors.New("model name cannot be whitespace only")
	}

	parsedURL, err := url.Parse(endpointURL)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return errors.New("endpoint URL must have scheme and host")
	}

	return nil
}

func (r *MemoryModelRegistry) removeEndpointFromIndex(endpointURL string) {
	endpointData, ok := r.endpointModels.Load(endpointURL)
	if ok {
		for _, model := range endpointData.Models {
			endpointSet, exists := r.modelToEndpoints.Load(model.Name)
			if exists {
				endpointSet.Delete(endpointURL)
				// Check if the set is now empty and remove it
				isEmpty := true
				endpointSet.Range(func(_ string, _ struct{}) bool {
					isEmpty = false
					return false // Stop after first item
				})
