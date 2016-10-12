package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/logger"
)

// recordHandler keeps every record so a test can count lines by message.
type recordHandler struct {
	mu   sync.Mutex
	msgs []string
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.msgs = append(h.msgs, r.Message)
	h.mu.Unlock()
	return nil
}
func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(string) slog.Handler      { return h }

func (h *recordHandler) count(msg string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, m := range h.msgs {
		if m == msg {
			n++
		}
	}
	return n
}

type warnCapture struct {
	*mockStyledLogger
	mu    sync.Mutex
	warns []string
}

func (w *warnCapture) Warn(msg string, _ ...any) {
	w.mu.Lock()
	w.warns = append(w.warns, msg)
	w.mu.Unlock()
}

func TestCombinedLogging_RejectionsAreSampledPerClient(t *testing.T) {
	t.Parallel()

	status := http.StatusForbidden
	h := &recordHandler{}
	log := &warnCapture{mockStyledLogger: &mockStyledLogger{underlying: slog.New(h)}}
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	mw := combinedLoggingMiddleware(log, clock, nil)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
