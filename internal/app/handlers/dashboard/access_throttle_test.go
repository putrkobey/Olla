package dashboard

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
)

// warnRecorder embeds stubLogger and records Warn lines.
type warnRecorder struct {
	stubLogger
	mu    sync.Mutex
	lines []warnLine
}

type warnLine struct {
	attrs map[string]any
	msg   string
}

func (w *warnRecorder) Warn(msg string, args ...any) {
	l := warnLine{msg: msg, attrs: map[string]any{}}
	for i := 0; i+1 < len(args); i += 2 {
		l.attrs[args[i].(string)] = args[i+1]
	}
	w.mu.Lock()
	w.lines = append(w.lines, l)
	w.mu.Unlock()
}

func (w *warnRecorder) snapshot() []warnLine {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]warnLine(nil), w.lines...)
}

// A blocked client can retry at wire speed: every request still gets the
