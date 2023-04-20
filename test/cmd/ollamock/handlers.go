package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// errUnknownMode is returned when a PATCH body sets an unrecognised mode.
var errUnknownMode = errors.New("unknown mode")

// Precompiled once at package init rather than per call inside modelMetadata -
// these run on every /api/tags response and the quantisation ones were
// previously compiled twice each (once for MatchString, again for FindString).
var (
	paramSizeRe = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*([bm])`)
	quantKLMSRe = regexp.MustCompile(`(?i)q\d+_k_[lms]`)
	quantNumRe  = regexp.MustCompile(`(?i)q\d+_\d+`)
	quantBareRe = regexp.MustCompile(`(?i)\bq\d+\b`)
	quantF16Re  = regexp.MustCompile(`(?i)fp?16`)
	quantF8Re   = regexp.MustCompile(`(?i)fp?8`)
)

// serverConfig holds static configuration set at startup.
// Field order largest-to-smallest for alignment.
type serverConfig struct {
	name string
	// teiKind is the model_type /info reports and the TEI routes enforce:
	// teiKindEmbedding (also the zero value), teiKindReranker or teiKindClassifier.
	teiKind string
	models  []string
	// loaded names the models /api/ps reports as resident. Empty means none.
	loaded       []string
	ttftMS       int
	tps          int
	streamChunks int
	// tagsCapabilities makes /api/tags carry per-model capabilities, as Ollama 0.30+
	// does. Off (the default) keeps the pre-existing listing for the main fleet.
	tagsCapabilities bool
}

// mockServer is the central state container wired into the HTTP mux.
type mockServer struct {
	bstate *behaviourState
	stats  statsStore
	cfg    serverConfig
}

func newServer(cfg serverConfig) *mockServer {
	// Seed from FNV-1a hash of the instance name so behaviour is deterministic
	// per-instance but distinct across instances in a multi-node test fleet.
	h := fnv.New64a()
	_, _ = h.Write([]byte(cfg.name))
	seed := int64(h.Sum64())

	return &mockServer{
		cfg:    cfg,
		bstate: newBehaviourState(seed),
	}
}

// handler builds and returns the ServeMux for this server instance.
func (srv *mockServer) handler() http.Handler {
	mux := http.NewServeMux()

	// Control plane - always healthy, immune to behaviour.
	srv.controlHandlers(mux)

	// LLM protocol routes.
	mux.HandleFunc("GET /health", srv.wrap(srv.handleHealth))
	mux.HandleFunc("GET /", srv.wrap(srv.handleRoot))

	// Model listing endpoints - one per protocol.
	mux.HandleFunc("GET /v1/models", srv.wrap(srv.handleOpenAIModels))
	mux.HandleFunc("GET /api/tags", srv.wrap(srv.handleOllamaTags))
	mux.HandleFunc("GET /api/version", srv.wrap(srv.handleOllamaVersion))
	mux.HandleFunc("GET /api/ps", srv.wrap(srv.handleOllamaPS))
	mux.HandleFunc("GET /api/v0/models", srv.wrap(srv.handleLMStudioModels))
	mux.HandleFunc("GET /api/v1/models", srv.wrap(srv.handleLemonadeModels))

	// Text Embeddings Inference (TEI) protocol - single model per instance,
	// discovered via /info rather than a model list. Native request bodies
	// carry no model field; response shapes mirror the OpenAPI spec closely
	// enough for forwarding tests.
	mux.HandleFunc("GET /info", srv.wrap(srv.handleTEIInfo))
	mux.HandleFunc("POST /v1/embeddings", srv.wrap(srv.handleTEIEmbeddings))
	mux.HandleFunc("POST /embed", srv.wrap(srv.handleTEIEmbed))
	mux.HandleFunc("POST /embed_all", srv.wrap(srv.handleTEIEmbedAll))
	mux.HandleFunc("POST /embed_sparse", srv.wrap(srv.handleTEIEmbedSparse))
	mux.HandleFunc("POST /tokenize", srv.wrap(srv.handleTEITokenize))
	mux.HandleFunc("POST /decode", srv.wrap(srv.handleTEIDecode))
	mux.HandleFunc("POST /predict", srv.wrap(srv.handleTEIPredict))
	mux.HandleFunc("POST /rerank", srv.wrap(srv.handleTEIRerank))
	mux.HandleFunc("POST /similarity", srv.wrap(srv.handleTESimilarity))
	mux.HandleFunc("GET /metrics", srv.wrap(srv.handleTEIMetrics))

	// Inference endpoints - streaming handled in streaming.go.
	mux.HandleFunc("POST /api/chat", srv.wrap(srv.handleOllamaChat))
	mux.HandleFunc("POST /api/generate", srv.wrap(srv.handleOllamaGenerate))
	mux.HandleFunc("POST /api/embed", srv.wrap(srv.handleOllamaEmbed))
	mux.HandleFunc("POST /v1/chat/completions", srv.wrap(srv.handleOpenAIChat))
	mux.HandleFunc("POST /v1/completions", srv.wrap(srv.handleOpenAICompletion))
	mux.HandleFunc("POST /api/v1/chat/completions", srv.wrap(srv.handleOpenAIChat))
	mux.HandleFunc("POST /v1/messages", srv.wrap(srv.handleAnthropicMessages))

	return mux
}

// wrap returns a handler that records stats, sets the instance header, and
// applies the current behaviour before delegating to the inner handler.
func (srv *mockServer) wrap(inner http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		srv.stats.inc(r.URL.Path)
		w.Header().Set("X-Ollamock-Instance", srv.cfg.name)
		if !srv.applyBehaviour(w, r) {
			return
		}
		inner(w, r)
	}
}

// applyBehaviour enforces the current Mode, returning false when the request
// should be short-circuited (response already written).
func (srv *mockServer) applyBehaviour(w http.ResponseWriter, r *http.Request) bool {
	b := srv.bstate.get()

	isHealthPath := r.URL.Path == "/health" || r.URL.Path == "/"

	// fail_health only gates the health endpoints - all other routes are unaffected.
	if b.FailHealth && isHealthPath {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"status":"unhealthy"}`)
		return false
	}

	switch b.Mode {
	case ModeOK:
		// No-op - proceed to handler.

	case ModeHang:
		// Block until the client disconnects or context is cancelled.
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Minute):
		}
		return false

	case ModeError:
		writeErrorResponse(w, b.ErrorStatus)
		return false

	case ModeFlaky:
		if srv.bstate.float64() < b.ErrorRate {
			writeErrorResponse(w, b.ErrorStatus)
			return false
		}

	case ModeSlow:
		if b.LatencyMS > 0 {
			select {
			case <-r.Context().Done():
				return false
			case <-time.After(time.Duration(b.LatencyMS) * time.Millisecond):
			}
		}
	}

	if b.MalformedJSON {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"broken":`)
		return false
	}

	return true
}

func writeErrorResponse(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"error":{"message":"mock error","type":"mock_error"}}`)
}

// writeJSON serialises v as JSON and writes it with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- Health / liveness ---

func (srv *mockServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (srv *mockServer) handleRoot(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "Ollama is running")
}

// --- Ollama protocol ---

func (srv *mockServer) handleOllamaTags(w http.ResponseWriter, _ *http.Request) {
	type details struct {
		Family            string `json:"family"`
		ParameterSize     string `json:"parameter_size"`
		QuantizationLevel string `json:"quantization_level"`
	}
	type model struct {
		Details    details `json:"details"`
		ModifiedAt string  `json:"modified_at"`
		Digest     string  `json:"digest"`
		Name       string  `json:"name"`
		Model      string  `json:"model"`
		// omitted entirely when empty, which is what older Ollama sends
		Capabilities []string `json:"capabilities,omitempty"`
		Size         int64    `json:"size"`
	}
	type response struct {
		Models []model `json:"models"`
	}

	now := time.Now().UTC().Format(time.RFC3339)
	models := make([]model, len(srv.cfg.models))
	for i, m := range srv.cfg.models {
		fam, pdisp, billions, q := modelMetadata(m)
		models[i] = model{
			Name:       m,
			Model:      m,
			ModifiedAt: now,
			Size:       sizeBytes(billions, q),
			Digest:     fmt.Sprintf("sha256:ollamock%s%s", srv.cfg.name, m),
			Details: details{
				Family:            fam,
				ParameterSize:     pdisp,
				QuantizationLevel: q,
			},
		}
		if srv.cfg.tagsCapabilities {
			models[i].Capabilities = ollamaCapabilities(m)
		}
	}
	writeJSON(w, http.StatusOK, response{Models: models})
}

// handleOllamaPS reports the models named by --loaded as resident, in Ollama's
// /api/ps shape. Loaded state is per instance in real Ollama too, which is what
// makes a merge across instances worth testing.
func (srv *mockServer) handleOllamaPS(w http.ResponseWriter, _ *http.Request) {
	type model struct {
		Details       map[string]string `json:"details"`
		ExpiresAt     string            `json:"expires_at"`
		Name          string            `json:"name"`
		Model         string            `json:"model"`
		Digest        string            `json:"digest"`
		Runner        string            `json:"runner"`
		Size          int64             `json:"size"`
		SizeVRAM      int64             `json:"size_vram"`
		ContextLength int               `json:"context_length"`
	}

	expires := time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339)
	models := make([]model, 0, len(srv.cfg.loaded))
	for _, m := range srv.cfg.loaded {
		fam, pdisp, billions, q := modelMetadata(m)
		size := sizeBytes(billions, q)
		models = append(models, model{
			Name:      m,
			Model:     m,
			Size:      size,
			SizeVRAM:  size,
			ExpiresAt: expires,
			Digest:    fmt.Sprintf("sha256:ollamock%s%s", srv.cfg.name, m),
			Details:   map[string]string{"family": fam, "parameter_size": pdisp, "quantization_level": q},
			// Ollama reports the loaded context window and the runner serving it
			// (api.ProcessModelResponse); fixed plausible values, as for size.
			ContextLength: 4096,
			Runner:        "ggml",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// handleOllamaEmbed answers Ollama's native /api/embed: "input" is a string or an
// array of strings and the reply carries one vector per input. It reuses the TEI
// deterministic vectors so tests can tell which instance answered by header, not
// by content. Like Ollama's EmbedHandler, an unknown model is a 404 and empty or
// absent input is a 200 with an empty embeddings array, not an error. A request
// with no model at all is still served: the harness sends model-less bodies to
// check routing.
func (srv *mockServer) handleOllamaEmbed(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model string          `json:"model"`
		Input json.RawMessage `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Model != "" && len(srv.cfg.models) > 0 && !srv.hasOllamaModel(req.Model) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": fmt.Sprintf("model '%s' not found", req.Model)})
		return
	}
	count, ok := ollamaEmbedInputCount(req.Input)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input type"})
		return
	}
	embeddings := make([][]float64, count)
	for i := range embeddings {
		embeddings[i] = teiVector(i)
	}
	resp := map[string]any{"model": req.Model, "embeddings": embeddings}
	if count > 0 {
		resp["total_duration"] = 1000000
		resp["load_duration"] = 500000
		resp["prompt_eval_count"] = count
	}
	writeJSON(w, http.StatusOK, resp)
}

// ollamaEmbedInputCount counts the inputs of an /api/embed body. Absent, null and
