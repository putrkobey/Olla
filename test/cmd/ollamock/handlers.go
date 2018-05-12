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
// "" all mean none; ok is false for any other type.
func ollamaEmbedInputCount(raw json.RawMessage) (count int, ok bool) {
	if !teiPresent(raw) {
		return 0, true
	}
	var in any
	if err := json.Unmarshal(raw, &in); err != nil {
		return 0, false
	}
	switch v := in.(type) {
	case string:
		if v == "" {
			return 0, true
		}
		return 1, true
	case []any:
		for _, item := range v {
			if _, isStr := item.(string); !isStr {
				return 0, false
			}
		}
		return len(v), true
	}
	return 0, false
}

// hasOllamaModel reports whether the instance serves name, treating a bare name
// as :latest the way Ollama does.
func (srv *mockServer) hasOllamaModel(name string) bool {
	withTag := func(m string) string {
		if !strings.Contains(m, ":") {
			return m + ":latest"
		}
		return m
	}
	return slices.ContainsFunc(srv.cfg.models, func(m string) bool { return withTag(m) == withTag(name) })
}

// embedderNameHints are substrings that mark a model name as an embedder. The
// mock has no model files to inspect, so the name stands in for what Ollama
// reads from the model's architecture.
var embedderNameHints = []string{"embed", "minilm", "bge-", "mxbai", "gte-", "e5-", "stella", "msmarco", "multi-qa"}

// Name hints for the capabilities beyond embedding. As with embedders, the name
// stands in for what Ollama reads from the model's template and projector.
var (
	visionNameHints = []string{"-vl", "llava", "vision", "gemma3", "minicpm-v"}
	toolsNameHints  = []string{"llama3.1", "llama3.2", "llama3.3", "qwen3", "qwen2.5", "mistral", "-vl"}
)

// ollamaCapabilities returns the capabilities Ollama 0.30+ lists for a model:
// "embedding" alone for embedders, otherwise "completion" plus "vision" and
// "tools" where the name suggests them. Order follows Ollama's: completion first.
func ollamaCapabilities(name string) []string {
	lower := strings.ToLower(name)
	if containsAny(lower, embedderNameHints) {
		return []string{"embedding"}
	}
	caps := []string{"completion"}
	if containsAny(lower, visionNameHints) {
		caps = append(caps, "vision")
	}
	if containsAny(lower, toolsNameHints) {
		caps = append(caps, "tools")
	}
	return caps
}

func containsAny(s string, hints []string) bool {
	return slices.ContainsFunc(hints, func(h string) bool { return strings.Contains(s, h) })
}

// modelMetadata derives a plausible family, parameter size and quantisation
// from a model name so the Ollama tags listing reflects the actual model
// rather than one hardcoded 7B/Q4_0 value for everything. Lets the mock back
// a realistic-looking fleet for screenshots and demos.
func modelMetadata(name string) (family, paramDisplay string, billions float64, quant string) {
	lower := strings.ToLower(name)
	if idx := strings.LastIndex(lower, "/"); idx != -1 {
		lower = lower[idx+1:]
	}
	family = "llama"
	for _, f := range []string{"deepseek", "llama", "gemma", "qwen", "mistral", "mixtral", "phi", "nomic", "yi", "falcon", "kimi", "granite", "glm", "starcoder", "codellama"} {
		if strings.HasPrefix(lower, f) {
			family = f
			break
		}
	}

	// Parameter size: first "<digits>[.<digits>]<b|m>" token in the name, case
	// insensitive so names like "Llama-3-8B" match (upstream naming isn't
	// consistent about casing here).
	if ms := paramSizeRe.FindStringSubmatch(name); ms != nil {
		num, err := strconv.ParseFloat(ms[1], 64)
		// Config-sourced input only (never request-controlled), but guard
		// against Inf/NaN anyway - sizeBytes would otherwise propagate them
		// into the response.
		if err == nil && !math.IsInf(num, 0) && !math.IsNaN(num) {
			if strings.EqualFold(ms[2], "m") {
				paramDisplay = fmt.Sprintf("%.0fM", num)
				billions = num / 1000
			} else {
				paramDisplay = fmt.Sprintf("%gB", num)
				billions = num
			}
		}
	}

	// Quantisation from the name when present, else the common Ollama pull default.
	quant = "Q4_K_M"
	switch {
	case quantKLMSRe.MatchString(name):
		quant = strings.ToUpper(quantKLMSRe.FindString(name))
	case quantNumRe.MatchString(name):
		quant = strings.ToUpper(quantNumRe.FindString(name))
	case quantBareRe.MatchString(name):
		quant = strings.ToUpper(quantBareRe.FindString(name))
	case quantF16Re.MatchString(name):
		quant = "F16"
	case quantF8Re.MatchString(name):
		quant = "F8"
	}
	return family, paramDisplay, billions, quant
}

// sizeBytes approximates on-disk size from a parameter count and quantisation.
func sizeBytes(billions float64, quant string) int64 {
	if billions <= 0 {
		return 0
	}
	bpp := 0.56 // bytes per parameter at Q4_K_M
	switch {
	case strings.HasPrefix(quant, "Q2"):
		bpp = 0.30
	case strings.HasPrefix(quant, "Q3"):
		bpp = 0.42
	case strings.HasPrefix(quant, "Q4"):
		bpp = 0.56
	case strings.HasPrefix(quant, "Q5"):
		bpp = 0.70
	case strings.HasPrefix(quant, "Q6"):
		bpp = 0.84
	case strings.HasPrefix(quant, "Q8"):
		bpp = 1.00
	case strings.HasPrefix(quant, "F16"), strings.HasPrefix(quant, "FP16"):
		bpp = 2.00
	case strings.HasPrefix(quant, "F32"), strings.HasPrefix(quant, "FP32"):
		bpp = 4.00
	}
	return int64(billions * bpp * float64(1_000_000_000))
}

func (srv *mockServer) handleOllamaVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": "0.6.0-ollamock"})
}

// --- LM Studio protocol ---

func (srv *mockServer) handleLMStudioModels(w http.ResponseWriter, _ *http.Request) {
	type model struct {
		Type             string `json:"type"`
		Publisher        string `json:"publisher"`
		Arch             string `json:"arch"`
		State            string `json:"state"`
		ID               string `json:"id"`
		Object           string `json:"object"`
		MaxContextLength int64  `json:"max_context_length"`
	}
	type response struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}

	models := make([]model, len(srv.cfg.models))
	for i, m := range srv.cfg.models {
		models[i] = model{
			ID:               m,
			Object:           "model",
			Type:             "llm",
			Publisher:        "ollamock",
			Arch:             "llama",
			State:            "loaded",
			MaxContextLength: 4096,
		}
	}
	writeJSON(w, http.StatusOK, response{Object: "list", Data: models})
}

// --- OpenAI-compatible protocol ---

func (srv *mockServer) handleOpenAIModels(w http.ResponseWriter, _ *http.Request) {
	type model struct {
		OwnedBy string `json:"owned_by"`
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
	}
	type response struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}

	now := time.Now().Unix()
	models := make([]model, len(srv.cfg.models))
	for i, m := range srv.cfg.models {
		models[i] = model{
			ID:      m,
			Object:  "model",
			Created: now,
			OwnedBy: "ollamock",
		}
	}
	writeJSON(w, http.StatusOK, response{Object: "list", Data: models})
}

// --- Lemonade protocol ---

func (srv *mockServer) handleLemonadeModels(w http.ResponseWriter, _ *http.Request) {
	type model struct {
		ID         string `json:"id"`
		Object     string `json:"object"`
		OwnedBy    string `json:"owned_by"`
		Checkpoint string `json:"checkpoint"`
		Recipe     string `json:"recipe"`
		Created    int64  `json:"created"`
		Downloaded bool   `json:"downloaded"`
	}
	type response struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}

	now := time.Now().Unix()
	models := make([]model, len(srv.cfg.models))
	for i, m := range srv.cfg.models {
		models[i] = model{
			ID:         m,
			Object:     "model",
			OwnedBy:    "lemonade",
			Checkpoint: fmt.Sprintf("amd/%s", m),
			Recipe:     "oga-cpu",
			Created:    now,
			Downloaded: true,
		}
	}
	writeJSON(w, http.StatusOK, response{Object: "list", Data: models})
}

// --- Text Embeddings Inference (TEI) protocol ---

// teiServedModel returns the model this instance reports as served. TEI runs
// exactly one embedding model per instance, so the first configured model is
// used regardless of how many names the instance was started with.
func (srv *mockServer) teiServedModel() string {
	if len(srv.cfg.models) == 0 {
		return ""
	}
	return srv.cfg.models[0]
}

// TEI model kinds, as spelled in /info's model_type tag.
const (
	teiKindEmbedding  = "embedding"
	teiKindReranker   = "reranker"
	teiKindClassifier = "classifier"
)

// teiKindOrDefault is the kind this instance serves. TEI runs one model per
// instance and the zero value means an embedding model.
func (srv *mockServer) teiKindOrDefault() string {
	if srv.cfg.teiKind == "" {
		return teiKindEmbedding
	}
	return srv.cfg.teiKind
}

// handleTEIInfo reports model_type as TEI does: an externally tagged object whose
// single key is the kind. Classifiers and rerankers carry their label maps.
func (srv *mockServer) handleTEIInfo(w http.ResponseWriter, _ *http.Request) {
	served := srv.teiServedModel()
	var modelType map[string]any
	switch srv.teiKindOrDefault() {
	case teiKindReranker:
		modelType = map[string]any{teiKindReranker: map[string]any{
			"id2label": map[string]string{"0": "LABEL_0"},
			"label2id": map[string]int{"LABEL_0": 0},
		}}
	case teiKindClassifier:
		modelType = map[string]any{teiKindClassifier: map[string]any{
			"id2label": map[string]string{"0": "negative", "1": "positive"},
			"label2id": map[string]int{"negative": 0, "positive": 1},
		}}
	default:
		modelType = map[string]any{teiKindEmbedding: map[string]string{"pooling": "cls"}}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"model_id":          "/data/" + strings.ReplaceAll(served, "/", "-"),
		"served_model_name": served,
		"model_dtype":       "float16",
		"max_input_length":  8192,
		"model_type":        modelType,
	})
}

// teiError is a TEI error response. TEI serialises ErrorType by variant name and
// maps it to a status: Empty is 400, Validation 422, Backend 424 (router/src/http/
// server.rs, router/src/lib.rs).
type teiError struct {
	errType string
	msg     string
	status  int
	// plain marks an axum extractor rejection: a text/plain body rather than the
	// JSON error envelope, because the request never reached TEI's own handler.
	plain bool
}

func teiEmptyErr(msg string) *teiError {
	return &teiError{status: http.StatusBadRequest, errType: "Empty", msg: msg}
}

// teiValidationErr is a body that parses as JSON but does not fit the request type
// (a missing field, a wrong JSON type). axum 0.7's Json extractor rejects it with a
// plain-text 422 before the handler runs, so there is no error_type to report.
func teiValidationErr(msg string) *teiError {
	return &teiError{status: http.StatusUnprocessableEntity, plain: true,
		msg: "Failed to deserialize the JSON body into the target type: " + msg}
}

// teiSyntaxErr is a body that is not JSON at all: axum answers a plain-text 400.
func teiSyntaxErr(msg string) *teiError {
	return &teiError{status: http.StatusBadRequest, plain: true,
		msg: "Failed to parse the request body as JSON: " + msg}
}

// teiWrongKindErr is what TEI answers when a route needs a different kind of
