package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/factory"
	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

const (
	// ollamaPSTimeout bounds the whole fan-out. /api/ps is answered from memory by
	// Ollama, so a healthy instance replies in milliseconds; anything slower is
	// better left out than allowed to hold the client.
	ollamaPSTimeout = 5 * time.Second

	// ollamaPSMaxBody caps what one instance can make us buffer. A real response is
	// a few hundred bytes per loaded model.
	ollamaPSMaxBody = 4 << 20

	// headerOllaPartial marks a merged answer that is missing instances.
	headerOllaPartial = constants.HeaderXOllaPartial

	// ollamaPSEndpointField names the instance a merged entry came from. Ollama's
	// own clients ignore unknown fields.
	ollamaPSEndpointField = "olla_endpoint"
)

var (
	errOllamaPSStatus = errors.New("unexpected status from /api/ps")
	errOllamaPSNoURL  = errors.New("endpoint has no parsed URL")
)

// ollamaRunningModelsHandler merges /api/ps across every healthy Ollama endpoint.
// endpoint: GET /olla/ollama/api/ps
//
// What is loaded is state held by each instance (not something Olla's registry
// tracks), so it has to be asked for live. Entries are kept per instance rather
// than de-duplicated: a model resident on two hosts occupies memory on both, and
// collapsing them would hide that. Each entry gains olla_endpoint naming its host.
// An instance that fails or times out is skipped and flagged with X-Olla-Partial;
// only when every instance fails is the whole request a 502.
func (a *Application) ollamaRunningModelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), ollamaPSTimeout)
	defer cancel()

	healthy, err := a.repository.GetHealthy(ctx)
	if err != nil {
		http.Error(w, "failed to list endpoints", http.StatusInternalServerError)
		return
	}

	var targets []*domain.Endpoint
