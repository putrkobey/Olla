package registry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func BenchmarkGetHealthyEndpointsForModel(b *testing.B) {
	ctx := context.Background()
	registry := createTestUnifiedRegistry()

	// Create mock endpoint repository
	endpoints := make([]*domain.Endpoint, 100)
	for i := range 100 {
		status := domain.StatusHealthy
		if i%3 == 0 {
			status = domain.StatusUnhealthy
		}
		endpoints[i] = &domain.Endpoint{
			URLString: fmt.Sprintf("http://localhost:%d", 11434+i),
			Name:      fmt.Sprintf("endpoint-%d", i),
			Status:    status,
		}
	}

	mockRepo := &mockEndpointRepository{endpoints: endpoints}

	// Register models on endpoints
	models := []string{"llama3:8b", "mistral:7b", "phi:3b", "qwen:14b", "deepseek:32b"}
	for i, ep := range endpoints {
		// Register 1-3 models per endpoint
		numModels := (i % 3) + 1
		for j := range numModels {
			modelIdx := (i + j) % len(models)
			model := &domain.ModelInfo{
				Name:     models[modelIdx],
				LastSeen: time.Now(),
			}
			registry.RegisterModel(ctx, ep.URLString, model)
		}
	}

	// Benchmark different scenarios
	b.Run("ExistingModel", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for range b.N {
			endpoints, err := registry.GetHealthyEndpointsForModel(ctx, "llama3:8b", mockRepo)
			if err != nil {
				b.Fatal(err)
			}
			_ = endpoints
		}
	})

	b.Run("NonExistentModel", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		for range b.N {
			endpoints, err := registry.GetHealthyEndpointsForModel(ctx, "gpt-4", mockRepo)
			if err != nil {
				b.Fatal(err)
			}
			_ = endpoints
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			modelIdx := 0
			for pb.Next() {
				model := models[modelIdx%len(models)]
				endpoints, err := registry.GetHealthyEndpointsForModel(ctx, model, mockRepo)
				if err != nil {
					b.Fatal(err)
				}
				_ = endpoints
				modelIdx++
			}
		})
	})
}

func BenchmarkGetModelsByCapability(b *testing.B) {
	ctx := context.Background()
	registry := createTestUnifiedRegistry()

	// Create test models with various capabilities
	capabilities := [][]string{
		{"chat", "streaming"},
		{"embeddings"},
		{"chat", "vision", "streaming"},
		{"chat", "code", "function_calling"},
		{"text", "completion"},
		{"chat", "function_calling", "streaming"},
	}

	// Add 1000 models to test at scale
	for i := range 1000 {
		model := &domain.UnifiedModel{
			ID:           fmt.Sprintf("model-%d", i),
			Family:       fmt.Sprintf("family-%d", i%10),
			Capabilities: capabilities[i%len(capabilities)],
		}
		registry.globalUnified.Store(model.ID, model)
	}

	// Benchmark different capabilities
	testCases := []string{"chat", "embeddings", "vision", "code", "streaming", "unknown"}

