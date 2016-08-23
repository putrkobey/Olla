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

		p50, p95, p99 := rs.GetPercentiles()
		if p50 != 100 || p95 != 100 || p99 != 100 {
			t.Error("Single value should return same value for all percentiles")
		}
	})

	t.Run("Reset functionality", func(t *testing.T) {
		rs := NewReservoirSampler(10)

		for i := range 100 {
			rs.Add(int64(i))
		}

		rs.Reset()

		if rs.Count() != 0 {
			t.Error("Count should be 0 after reset")
		}

		p50, p95, p99 := rs.GetPercentiles()
		if p50 != 0 || p95 != 0 || p99 != 0 {
			t.Error("Percentiles should be 0 after reset")
		}
	})
}

func TestSimpleStatsTracker(t *testing.T) {
	t.Run("Basic functionality", func(t *testing.T) {
		st := NewSimpleStatsTracker()

		// Add values 10, 20, 30, 40, 50
		values := []int64{10, 20, 30, 40, 50}
		for _, v := range values {
			st.Add(v)
		}

		if st.Count() != 5 {
			t.Errorf("Expected count 5, got %d", st.Count())
		}

		if avg := st.GetAverage(); avg != 30 {
			t.Errorf("Expected average 30, got %d", avg)
		}

		if min := st.GetMin(); min != 10 {
