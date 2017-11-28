package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func BenchmarkRepository_GetAll(b *testing.B) {
	repo := NewStaticEndpointRepository()
	ctx := context.Background()

	// Setup test endpoints via config
	configs := make([]config.EndpointConfig, 100)
	for i := range 100 {
		port := 11434 + i
		configs[i] = config.EndpointConfig{
			Name:           fmt.Sprintf("bench-%d", i),
			URL:            fmt.Sprintf("http://localhost:%d", port),
			HealthCheckURL: "/health",
			ModelURL:       "/api/tags",
			Priority:       ptrInt(100),
			CheckInterval:  5 * time.Second,
			CheckTimeout:   2 * time.Second,
		}
	}

	repo.LoadFromConfig(ctx, configs)

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := repo.GetAll(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRepository_GetRoutable(b *testing.B) {
	repo := NewStaticEndpointRepository()
	ctx := context.Background()

	// Mix of statuses - 60% routable (healthy + busy + warming)
	configs := make([]config.EndpointConfig, 100)
	statuses := []domain.EndpointStatus{
		domain.StatusHealthy,
		domain.StatusBusy,
