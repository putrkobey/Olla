package balancer

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/logger"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func createTestFactoryLogger() logger.StyledLogger {
	loggerCfg := &logger.Config{Level: "error", Theme: "default"}
	log, _, _ := logger.New(loggerCfg)
	return logger.NewPlainStyledLogger(log)
}
func NewTestStatsCollector() *stats.Collector {
	return stats.NewCollector(createTestFactoryLogger())
}
func BenchmarkFactory_Create(b *testing.B) {
	factory := NewFactory(NewTestStatsCollector())

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		selector, err := factory.Create(DefaultBalancerPriority)
		if err != nil {
			b.Fatal(err)
		}
		_ = selector
	}
}

func BenchmarkPrioritySelector_Select(b *testing.B) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	// Create test endpoints with different prior
	endpoints := make([]*domain.Endpoint, 10)
	for i := range 10 {
		endpoints[i] = createBenchEndpoint(fmt.Sprintf("endpoint-%d", i), 11434+i, domain.StatusHealthy, 100+i*10)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := selector.Select(ctx, endpoints)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPrioritySelector_SelectSamePriority tests weighted selection
func BenchmarkPrioritySelector_SelectSamePriority(b *testing.B) {
	selector := NewPrioritySelector(NewTestStatsCollector())
	ctx := context.Background()

	// Same priority, different statuses for weighted selection
	endpoints := []*domain.Endpoint{
		createBenchEndpoint("healthy", 11434, domain.StatusHealthy, 100),
		createBenchEndpoint("busy", 11435, domain.StatusBusy, 100),
		createBenchEndpoint("warming", 11436, domain.StatusWarming, 100),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := selector.Select(ctx, endpoints)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRoundRobinSelector_Select tests round-robin performance
func BenchmarkRoundRobinSelector_Select(b *testing.B) {
	selector := NewRoundRobinSelector(NewTestStatsCollector())
	ctx := context.Background()

	endpoints := make([]*domain.Endpoint, 10)
	for i := range 10 {
		endpoints[i] = createBenchEndpoint(fmt.Sprintf("endpoint-%d", i), 11434+i, domain.StatusHealthy, 100)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, err := selector.Select(ctx, endpoints)
		if err != nil {
			b.Fatal(err)
		}
	}
}

