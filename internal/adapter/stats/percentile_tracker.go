package stats

import (
	"math/rand/v2"
	"sort"
	"sync"
)

// PercentileTracker is an interface for tracking latencies and calculating percentiles
type PercentileTracker interface {
	Add(value int64)
	GetPercentiles() (p50, p95, p99 int64)
	Count() int64
	Reset()
}

// ReservoirSampler implements reservoir sampling for memory-efficient percentile calculation
// It maintains a fixed-size sample of values, providing good statistical accuracy
// with bounded memory usage (typically 100-200 samples vs 1000)
type ReservoirSampler struct {
	samples     []int64
	sampleSize  int
	count       int64
	mu          sync.Mutex
	initialized bool
}

// NewReservoirSampler creates a new reservoir sampler with the specified sample size
func NewReservoirSampler(sampleSize int) *ReservoirSampler {
	if sampleSize <= 0 {
		sampleSize = 100 // Default to 100 samples
	}
	return &ReservoirSampler{
		sampleSize: sampleSize,
		samples:    make([]int64, 0, sampleSize), // Start with empty slice, grow as needed
	}
}

// Add adds a new latency value to the sampler
func (rs *ReservoirSampler) Add(value int64) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.count++

	// Fill reservoir array until it's full
	if len(rs.samples) < rs.sampleSize {
		rs.samples = append(rs.samples, value)
		return
	}

	// Use reservoir sampling algorithm for subsequent values
