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
