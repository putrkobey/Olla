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
