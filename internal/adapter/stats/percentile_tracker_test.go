package stats

import (
	"testing"
)

func TestReservoirSampler(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		rs := NewReservoirSampler(10)

		// Add some values
		for i := int64(1); i <= 20; i++ {
			rs.Add(i * 10)
		}

		if rs.Count() != 20 {
			t.Errorf("Expected count 20, got %d", rs.Count())
		}

		p50, p95, p99 := rs.GetPercentiles()
		if p50 == 0 || p95 == 0 || p99 == 0 {
			t.Error("Percentiles should not be zero")
		}

		// P50 should be less than or equal to P95 which should be less than or equal to P99
		// With small sample sizes, these might be equal
		if p50 > p95 || p95 > p99 {
			t.Errorf("Invalid percentile ordering: p50=%d, p95=%d, p99=%d", p50, p95, p99)
		}
	})

	t.Run("Empty sampler", func(t *testing.T) {
		rs := NewReservoirSampler(10)

		p50, p95, p99 := rs.GetPercentiles()
		if p50 != 0 || p95 != 0 || p99 != 0 {
			t.Error("Empty sampler should return zero percentiles")
		}
	})

	t.Run("Single value", func(t *testing.T) {
		rs := NewReservoirSampler(10)
		rs.Add(100)

