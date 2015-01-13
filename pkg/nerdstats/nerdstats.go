package nerdstats

import (
	"runtime"
	"runtime/debug"
	"time"

	"github.com/putrkobey/Olla/pkg/format"
)

/*
	NerdStats v2 provides a comprehensive snapshot of Go runtime statistics.
	It includes memory usage, garbage collection stats, goroutine counts etc.

	See: https://pkg.go.dev/runtime#MemStats for more details on the fields.

	Usage:
		stats := nerdstats.Snapshot(time.Now())
		fmt.Println(stats)

	v1: https://github.com/putrkobey/smash/blob/main/pkg/nerdstats/nerdstats.go
*/

type NerdStats struct {
	LastGC time.Time // Time of last GC

	// Debug info
	BuildInfo *debug.BuildInfo
	GoVersion string // Go version
	// Memory stats
	HeapAlloc    uint64 // Allocated heap memory in bytes
	HeapSys      uint64 // Heap memory obtained from OS
	HeapInuse    uint64 // Heap memory in use
	HeapReleased uint64 // Heap memory released to OS
	StackInuse   uint64 // Stack memory in use
	StackSys     uint64 // Stack memory obtained from OS
	TotalAlloc   uint64 // Total bytes allocated (cumulative)
	Mallocs      uint64 // Number of malloc operations
	Frees        uint64 // Number of free operations

	TotalGCTime   time.Duration // Total time spent in GC
	GCCPUFraction float64       // Fraction of CPU time used by GC

	// Goroutine stats
	NumGoroutines int   // Number of goroutines
	NumCgoCall    int64 // Number of cgo calls

	// Go stats
	NumCPU     int           // Number of logical CPUs
	GOMAXPROCS int           // Max OS threads for Go
	Uptime     time.Duration // Process uptime

	// GC stats
	NumGC uint32 // Number of GC cycles
}

func Snapshot(startTime time.Time) *NerdStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	stats := &NerdStats{
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapInuse:    m.HeapInuse,
		HeapReleased: m.HeapReleased,
		StackInuse:   m.StackInuse,
		StackSys:     m.StackSys,
		TotalAlloc:   m.TotalAlloc,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,

		// GC
		NumGC:         m.NumGC,
		GCCPUFraction: m.GCCPUFraction,

		// Goroutine
		NumGoroutines: runtime.NumGoroutine(),
		NumCgoCall:    runtime.NumCgoCall(),

		// Runtime
		NumCPU:     runtime.NumCPU(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		GoVersion:  runtime.Version(),
		Uptime:     time.Since(startTime),
	}

	// Calculate last GC time and total GC time
	if m.LastGC > 0 {
		stats.LastGC = time.Unix(0, int64(m.LastGC))
		stats.TotalGCTime = time.Duration(m.PauseTotalNs)
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		stats.BuildInfo = info
	}

	return stats
}

// GetMemoryPressure returns a simple assessment of memory pressure
func (ps *NerdStats) GetMemoryPressure() string {
	heapUsageRatio := float64(ps.HeapInuse) / float64(ps.HeapSys)
	allocsPerFree := float64(ps.Mallocs) / float64(ps.Frees+1) // +1 to avoid division by zero

	if heapUsageRatio > 0.9 && allocsPerFree > 1.5 {
		return "HIGH"
	} else if heapUsageRatio > 0.7 || allocsPerFree > 1.2 {
		return "MEDIUM"
	}
	return "LOW"
}

// GetGoroutineHealthStatus assesses goroutine count health
func (ps *NerdStats) GetGoroutineHealthStatus() string {
	// These thresholds are conservative
	switch {
