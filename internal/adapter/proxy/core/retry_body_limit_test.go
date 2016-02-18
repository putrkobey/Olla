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
