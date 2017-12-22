package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// countingHandler counts Handle calls, discards output, and respects a fixed min level.
// Used to assert whether log records are emitted without depending on stdout output.
type countingHandler struct {
	minLevel slog.Level
	count    int
}

func (h *countingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.minLevel
}
func (h *countingHandler) Handle(_ context.Context, _ slog.Record) error {
	h.count++
	return nil
}
func (h *countingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *countingHandler) WithGroup(_ string) slog.Handler      { return h }

// setDefaultLogger replaces slog.Default for the duration of the test.
// NOT safe to call from t.Parallel() tests as slog.Default is process-global;
// these tests must run serially via subtests under a single parent.
func setDefaultLogger(t *testing.T, h slog.Handler) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// TestCombinedLogging_Gate exercises the level-gate logic in
// CombinedLoggingMiddleware, which now does both the console log (gated by
// isQuietPollRoute/isQuietPollOutcome) AND the access log (gated by
// accessLogLevel/isQuietAccessOutcome) in one pass - so ch.count here is the
// sum of both outputs, not just the console line's. Each subtest's comment
// breaks down console vs access counts explicitly since the two gates
// deliberately disagree on proxy traffic (console always quiets it, access
// never does).
// All subtests share the same slog.Default mutation so they must run serially.
func TestCombinedLogging_Gate(t *testing.T) {
	mockLogger := &mockStyledLogger{}
	noop := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// ProxyPath_InfoLevel: at info level, console Debug records must not be
	// emitted (0), but the access log is NOT quieted for proxy traffic - a
	// proxy success is the audit-worthy outcome, so it logs at Info (1).
	t.Run("ProxyPath_InfoLevel", func(t *testing.T) {
		ch := &countingHandler{minLevel: slog.LevelInfo}
		setDefaultLogger(t, ch)

		var gotRequestID string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotRequestID = GetRequestID(r.Context())
			w.WriteHeader(http.StatusOK)
