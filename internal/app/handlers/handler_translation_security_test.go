package handlers

// Regression tests for Phase 3 security hardening items:
//   2. Upstream error-body read is capped at MaxUpstreamErrorBodyBytes.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// TestStreamingBackendError_ErrorBodyCapped verifies that handleStreamingBackendError
// does not deadlock when the upstream sends more than MaxUpstreamErrorBodyBytes.
// Without io.LimitReader the handler would read indefinitely; with it, reading stops
// at the cap and the pipeReader is closed to unblock the proxy goroutine.
func TestStreamingBackendError_ErrorBodyCapped(t *testing.T) {
	t.Parallel()

	proxyFunc := func(
		_ context.Context,
		w http.ResponseWriter,
		_ *http.Request,
		_ []*domain.Endpoint,
		_ *ports.RequestStats,
		_ logger.StyledLogger,
	) error {
		w.WriteHeader(http.StatusInternalServerError)
		// Write exactly the cap so we sit on the LimitReader boundary.
		// The write may be cut short when the reader closes the pipe; that is
		// intentional — the proxy error is discarded in the error path.
		buf := make([]byte, constants.MaxUpstreamErrorBodyBytes)
		copy(buf, `{"error":{"message":"big-error"}}`)
		_, _ = w.Write(buf)
		return nil
	}

	trans := &mockTranslator{
		name:                  "cap-test-translator",
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
