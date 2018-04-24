package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/util"
	"github.com/putrkobey/Olla/pkg/format"
	"github.com/putrkobey/Olla/pkg/nerdstats"
)

type ProcessStatsResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Memory    struct {
		HeapAlloc      string `json:"heap_alloc"`
		HeapSys        string `json:"heap_sys"`
		HeapInuse      string `json:"heap_inuse"`
		HeapReleased   string `json:"heap_released"`
		StackInuse     string `json:"stack_inuse"`
		TotalAlloc     string `json:"total_alloc"`
		MemoryPressure string `json:"memory_pressure"`
	} `json:"memory"`

	GarbageCollection struct {
		LastGC        string  `json:"last_gc"`
		TotalGCTime   string  `json:"total_gc_time"`
		AvgGCPause    string  `json:"avg_gc_pause"`
		GCCPUFraction float64 `json:"gc_cpu_fraction"`
		NumGC         uint32  `json:"num_gc_cycles"`
	} `json:"garbage_collection"`

	Goroutines struct {
		HealthStatus string `json:"health_status"`
		Count        int    `json:"count"`
		CgoCalls     int64  `json:"cgo_calls"`
	} `json:"goroutines"`

	Runtime struct {
		Uptime     string `json:"uptime"`
		GoVersion  string `json:"go_version"`
		NumCPU     int    `json:"num_cpu"`
		GOMAXPROCS int    `json:"gomaxprocs"`
	} `json:"runtime"`

	Allocations struct {
		TotalMallocs uint64 `json:"total_mallocs"`
		TotalFrees   uint64 `json:"total_frees"`
		NetObjects   int64  `json:"net_objects"`
	} `json:"allocations"`
}

func (a *Application) processStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := nerdstats.Snapshot(a.StartTime)

	response := ProcessStatsResponse{
		Timestamp: time.Now(),
	}

	response.Memory.HeapAlloc = format.Bytes(stats.HeapAlloc)
	response.Memory.HeapSys = format.Bytes(stats.HeapSys)
	response.Memory.HeapInuse = format.Bytes(stats.HeapInuse)
	response.Memory.HeapReleased = format.Bytes(stats.HeapReleased)
	response.Memory.StackInuse = format.Bytes(stats.StackInuse)
	response.Memory.TotalAlloc = format.Bytes(stats.TotalAlloc)
