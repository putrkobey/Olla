package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type warnCounter struct {
	*mockStyledLogger
	mu     sync.Mutex
	counts map[string]int
	errors int
}

func newWarnCounter() *warnCounter {
	return &warnCounter{mockStyledLogger: &mockStyledLogger{}, counts: map[string]int{}}
}

func (w *warnCounter) Warn(msg string, _ ...any) {
	w.mu.Lock()
	w.counts[msg]++
	w.mu.Unlock()
}

// WithRequestID keeps the counter in place when a handler derives a per-request logger.
func (w *warnCounter) WithRequestID(string) logger.StyledLogger { return w }

func (w *warnCounter) Error(msg string, _ ...any) {
	w.mu.Lock()
	w.counts[msg]++
	w.errors++
	w.mu.Unlock()
}

// errorTotal is every ERROR written, whatever its message.
func (w *warnCounter) errorTotal() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.errors
}

func (w *warnCounter) count(msg string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.counts[msg]
}

func rejectedRequest(ip, model string) *proxyRequest {
	return &proxyRequest{
		requestLogger: nil, clientIP: ip, model: model, stats: &ports.RequestStats{StartTime: time.Now()},
	}
}

// A client repeating a refused request, or rotating the model name it asks for, must not
// write a line per attempt. A different client or status is not hidden by it.
func TestRejectLogs_RepeatedRefusalsAreSampled(t *testing.T) {
