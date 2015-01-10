package core

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/logger"
)

type errorCounter struct{ errors atomic.Int32 }

func (h *errorCounter) Enabled(context.Context, slog.Level) bool { return true }
func (h *errorCounter) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *errorCounter) WithGroup(string) slog.Handler            { return h }
func (h *errorCounter) Handle(_ context.Context, r slog.Record) error {
	if r.Level >= slog.LevelError {
		h.errors.Add(1)
	}
	return nil
}

// An oversized chunked body is answered 413 by the handler, which samples it. The retry
// layer must return the error without its own ERROR per request, while a genuine read
// failure still logs.
func TestPreserveRequestBody_MaxBytesErrorIsNotLogged(t *testing.T) {
	t.Parallel()
