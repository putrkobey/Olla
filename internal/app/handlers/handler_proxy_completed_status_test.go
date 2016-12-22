package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/theme"
)

// The per-request "Request completed" line is the operator's record of a proxied request,
// so it must say what status the client got and a backend 5xx must not read as a clean
// completion. These tests drive real engines over every route kind and read the log back.

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

type logLine struct {
	Level string `json:"level"`
	Msg   string `json:"msg"`
	// kept loose: slog encodes ints as JSON numbers, and "status" is a string on completed lines
	HTTPStatus *float64 `json:"http_status"`
	Status     any      `json:"status"`
}

func (b *lockedBuffer) lines(t *testing.T, msg string) []logLine {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []logLine
	for _, raw := range strings.Split(b.buf.String(), "\n") {
		if raw == "" {
			continue
		}
		var l logLine
		require.NoError(t, json.Unmarshal([]byte(raw), &l), raw)
		if l.Msg == msg {
			out = append(out, l)
		}
	}
	return out
}

type statusRoute struct {
	name    string
	handler string // proxy, provider, anthropic
	path    string
	// passthrough only matters for the anthropic route
	passthrough bool
}

