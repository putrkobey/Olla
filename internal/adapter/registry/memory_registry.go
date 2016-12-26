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
