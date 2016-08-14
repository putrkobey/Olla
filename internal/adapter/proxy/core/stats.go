package core

import (
	"sync/atomic"

	"github.com/putrkobey/Olla/internal/core/ports"
)

// ProxyStats contains common proxy statistics
type ProxyStats struct {
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	TotalLatency       int64
	MinLatency         int64
	MaxLatency         int64
}

// RecordSuccess records a successful proxy request
func (s *ProxyStats) RecordSuccess(latency int64) {
	atomic.AddInt64(&s.SuccessfulRequests, 1)
	atomic.AddInt64(&s.TotalLatency, latency)

	// Update min latency
	for {
		oldMin := atomic.LoadInt64(&s.MinLatency)
		if oldMin != 0 && oldMin <= latency {
			break
