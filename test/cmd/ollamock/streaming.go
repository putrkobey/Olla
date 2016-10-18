package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// inferenceRequest is the minimal request shape shared across all protocols.
// Stream nil means "use protocol default" - true for Ollama, false for OpenAI.
type inferenceRequest struct {
	Stream *bool  `json:"stream"`
	Model  string `json:"model"`
}

func parseInferenceRequest(r *http.Request) (inferenceRequest, error) {
	var req inferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		return inferenceRequest{}, err
	}
	return req, nil
}

// streamBool is true if stream==nil (use given default) or stream==value.
func resolveStream(s *bool, defaultStream bool) bool {
	if s == nil {
		return defaultStream
	}
	return *s
}

// --- Ollama chat / generate ---

func (srv *mockServer) handleOllamaChat(w http.ResponseWriter, r *http.Request) {
	var req inferenceRequest
	body := readBody(r)
	_ = json.Unmarshal(body, &req)

	model := req.Model
	if model == "" && len(srv.cfg.models) > 0 {
		model = srv.cfg.models[0]
	}

	stream := resolveStream(req.Stream, true) // Ollama streams by default

	if stream {
		srv.streamOllamaChat(w, r, model)
	} else {
		srv.nonStreamOllamaChat(w, model)
	}
}

func (srv *mockServer) handleOllamaGenerate(w http.ResponseWriter, r *http.Request) {
	var req inferenceRequest
	body := readBody(r)
	_ = json.Unmarshal(body, &req)

	model := req.Model
	if model == "" && len(srv.cfg.models) > 0 {
		model = srv.cfg.models[0]
	}

	stream := resolveStream(req.Stream, true)

	if stream {
		srv.streamOllamaGenerate(w, r, model)
	} else {
		srv.nonStreamOllamaGenerate(w, model)
	}
}

func (srv *mockServer) nonStreamOllamaChat(w http.ResponseWriter, model string) {
	now := time.Now().UTC().Format(time.RFC3339)
	resp := map[string]any{
		"model":      model,
		"created_at": now,
		"message": map[string]any{
			"role":    "assistant",
			"content": fmt.Sprintf("BACKEND:%s model:%s", srv.cfg.name, model),
		},
		"done":                 true,
		"prompt_eval_count":    10,
		"eval_count":           20,
		"total_duration":       1_000_000_000,
		"load_duration":        100_000_000,
		"prompt_eval_duration": 200_000_000,
		"eval_duration":        700_000_000,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *mockServer) nonStreamOllamaGenerate(w http.ResponseWriter, model string) {
	now := time.Now().UTC().Format(time.RFC3339)
	resp := map[string]any{
		"model":                model,
		"created_at":           now,
		"response":             fmt.Sprintf("BACKEND:%s model:%s", srv.cfg.name, model),
		"done":                 true,
		"prompt_eval_count":    10,
		"eval_count":           20,
		"total_duration":       1_000_000_000,
		"load_duration":        100_000_000,
		"prompt_eval_duration": 200_000_000,
		"eval_duration":        700_000_000,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *mockServer) streamOllamaChat(w http.ResponseWriter, r *http.Request, model string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	rc := http.NewResponseController(w)

	applyTTFT(r.Context(), srv.cfg.ttftMS)

	n := srv.cfg.streamChunks
	b := srv.bstate.get()
	dropAt := (n + 1) / 2 // ceil(n/2) for drop_mid_stream

	now := time.Now().UTC().Format(time.RFC3339)

	for i := range n {
		if b.DropMidStream && i == dropAt {
			// Truncate mid-stream - client gets EOF without a done:true final line.
			tryHijackClose(w)
			return
		}

		chunk := map[string]any{
			"model":      model,
			"created_at": now,
			"message": map[string]any{
				"role":    "assistant",
				"content": fmt.Sprintf("The answer is 42. BACKEND:%s model:%s chunk:%d", srv.cfg.name, model, i),
			},
			"done": false,
		}
		writeNDJSON(w, chunk)
		_ = rc.Flush()
		applyTPS(r.Context(), srv.cfg.tps)
	}

	// Final done:true line with metrics.
	final := map[string]any{
		"model":      model,
		"created_at": now,
		"message": map[string]any{
			"role":    "assistant",
			"content": "",
		},
		"done":                 true,
		"prompt_eval_count":    10,
		"eval_count":           20,
		"total_duration":       1_000_000_000,
		"load_duration":        100_000_000,
		"prompt_eval_duration": 200_000_000,
		"eval_duration":        700_000_000,
	}
	writeNDJSON(w, final)
	_ = rc.Flush()
}

func (srv *mockServer) streamOllamaGenerate(w http.ResponseWriter, r *http.Request, model string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	rc := http.NewResponseController(w)

	applyTTFT(r.Context(), srv.cfg.ttftMS)

	n := srv.cfg.streamChunks
	b := srv.bstate.get()
	dropAt := (n + 1) / 2
	now := time.Now().UTC().Format(time.RFC3339)

	for i := range n {
		if b.DropMidStream && i == dropAt {
			tryHijackClose(w)
			return
		}

		chunk := map[string]any{
			"model":      model,
			"created_at": now,
			"response":   fmt.Sprintf("The answer is 42. BACKEND:%s model:%s chunk:%d", srv.cfg.name, model, i),
			"done":       false,
		}
		writeNDJSON(w, chunk)
		_ = rc.Flush()
		applyTPS(r.Context(), srv.cfg.tps)
	}

	final := map[string]any{
		"model":                model,
		"created_at":           now,
		"response":             "",
		"done":                 true,
		"prompt_eval_count":    10,
		"eval_count":           20,
		"total_duration":       1_000_000_000,
		"load_duration":        100_000_000,
		"prompt_eval_duration": 200_000_000,
		"eval_duration":        700_000_000,
	}
	writeNDJSON(w, final)
	_ = rc.Flush()
}

// --- OpenAI chat completions ---

func (srv *mockServer) handleOpenAIChat(w http.ResponseWriter, r *http.Request) {
	var req inferenceRequest
	body := readBody(r)
	_ = json.Unmarshal(body, &req)

	model := req.Model
	if model == "" && len(srv.cfg.models) > 0 {
		model = srv.cfg.models[0]
	}

	stream := resolveStream(req.Stream, false) // OpenAI defaults to non-streaming

	if stream {
		srv.streamOpenAIChat(w, r, model)
	} else {
		srv.nonStreamOpenAIChat(w, model)
	}
}

func (srv *mockServer) nonStreamOpenAIChat(w http.ResponseWriter, model string) {
	now := time.Now().Unix()
	resp := map[string]any{
		"id":      fmt.Sprintf("chatcmpl-mock-%s", srv.cfg.name),
		"object":  "chat.completion",
		"created": now,
		"model":   model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": fmt.Sprintf("BACKEND:%s reply", srv.cfg.name),
				},
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     10,
			"completion_tokens": 20,
			"total_tokens":      30,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *mockServer) streamOpenAIChat(w http.ResponseWriter, r *http.Request, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	rc := http.NewResponseController(w)

	applyTTFT(r.Context(), srv.cfg.ttftMS)

	id := fmt.Sprintf("chatcmpl-mock-%s", srv.cfg.name)
	now := time.Now().Unix()
	n := srv.cfg.streamChunks
	b := srv.bstate.get()
	dropAt := (n + 1) / 2

	for i := range n {
		if b.DropMidStream && i == dropAt {
			tryHijackClose(w)
			return
		}

		chunk := map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": now,
			"model":   model,
			"choices": []map[string]any{
				{
					"index": 0,
					"delta": map[string]any{
						"role":    "assistant",
						"content": fmt.Sprintf("BACKEND:%s reply: chunk %d", srv.cfg.name, i),
					},
					"finish_reason": nil,
				},
			},
		}
		data, _ := json.Marshal(chunk)
		writeSSEData(w, data)
		_ = rc.Flush()
		applyTPS(r.Context(), srv.cfg.tps)
	}

	// Final chunk signals stop.
	final := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": now,
		"model":   model,
		"choices": []map[string]any{
			{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			},
		},
	}
	data, _ := json.Marshal(final)
	writeSSEData(w, data)
	_ = rc.Flush()

	// Terminal usage chunk in the include_usage shape: choices:[] with populated usage.
	// Matches the non-stream path token numbers so streaming and non-streaming agree.
	// This exercises the choices:[] usage-capture path in the translation layer and
	// gives the translator a real output_tokens value instead of falling back to the
	// chars/4 estimate.
