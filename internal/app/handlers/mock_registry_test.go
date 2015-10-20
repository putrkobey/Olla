package handlers

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// baseMockRegistry provides default implementations for all ModelRegistry methods
type baseMockRegistry struct{}

func (m *baseMockRegistry) RegisterModel(ctx context.Context, endpointURL string, model *domain.ModelInfo) error {
	return nil
}

func (m *baseMockRegistry) RegisterModels(ctx context.Context, endpointURL string, models []*domain.ModelInfo) error {
	return nil
}

func (m *baseMockRegistry) GetModelsForEndpoint(ctx context.Context, endpointURL string) ([]*domain.ModelInfo, error) {
	return nil, nil
}

func (m *baseMockRegistry) GetEndpointsForModel(ctx context.Context, modelName string) ([]string, error) {
	return []string{}, nil
}

func (m *baseMockRegistry) IsModelAvailable(ctx context.Context, modelName string) bool {
	return false
}

func (m *baseMockRegistry) GetAllModels(ctx context.Context) (map[string][]*domain.ModelInfo, error) {
	return nil, nil
}
