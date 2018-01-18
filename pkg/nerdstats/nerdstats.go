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
