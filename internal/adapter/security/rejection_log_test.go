package security

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// captureHandler records every log record so tests can count lines and read
// their attributes.
type captureHandler struct {
	mu      *sync.Mutex
	records *[]capturedRecord
}

type capturedRecord struct {
	attrs map[string]any
	msg   string
}

func newCaptureLogger() (logger.StyledLogger, func() []capturedRecord) {
	h := &captureHandler{mu: &sync.Mutex{}, records: &[]capturedRecord{}}
	snapshot := func() []capturedRecord {
		h.mu.Lock()
		defer h.mu.Unlock()
		return append([]capturedRecord(nil), *h.records...)
	}
	return logger.NewPlainStyledLogger(slog.New(h)), snapshot
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *captureHandler) WithGroup(string) slog.Handler            { return h }
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{msg: r.Message, attrs: map[string]any{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	*h.records = append(*h.records, rec)
	h.mu.Unlock()
	return nil
}

func countExact(recs []capturedRecord, msg string) (n int) {
	for _, r := range recs {
		if r.msg == msg {
			n++
		}
	}
	return n
}

func countMsg(recs []capturedRecord, prefix string) (n int) {
	for _, r := range recs {
		if strings.HasPrefix(r.msg, prefix) {
			n++
		}
	}
	return n
}

type fakeClock struct {
	t  time.Time
	mu sync.Mutex
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

type countingMetrics struct {
	violations map[string]int
	mu         sync.Mutex
