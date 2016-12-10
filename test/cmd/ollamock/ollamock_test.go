package main

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testServer creates an isolated httptest.Server for each test, so tests never
// share mutable state. The server is closed automatically via t.Cleanup.
func testServer(t *testing.T, models ...string) *httptest.Server {
	t.Helper()
	if len(models) == 0 {
		models = []string{"test-model"}
	}
	cfg := serverConfig{
		name:         "test",
		models:       models,
		ttftMS:       0,
		tps:          0,
		streamChunks: 3,
	}
	srv := newServer(cfg)
	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	return ts
}

// postJSON sends a POST with a JSON body and returns the response.
func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// getURL sends a GET and returns the response.
func getURL(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// decodeJSON decodes the response body into v, closing the body.
func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// --- Tests ---

// TestModelMetadataCaseInsensitiveParamSize covers the regression: the param
// size regex used to be case-sensitive, so a model named with an uppercase
// unit suffix (as real Ollama library names commonly are, e.g. "Llama-3-8B")
// matched nothing, leaving paramDisplay empty and sizeBytes 0.
func TestModelMetadataCaseInsensitiveParamSize(t *testing.T) {
	t.Parallel()

	_, paramDisplay, billions, _ := modelMetadata("Llama-3-8B")
	if paramDisplay != "8B" {
		t.Errorf("paramDisplay = %q, want 8B", paramDisplay)
	}
	if billions != 8 {
		t.Errorf("billions = %v, want 8", billions)
	}
}

// TestModelMetadataNonFiniteGuard covers the non-finite ParseFloat guard: a
// name whose digits parse to Inf must not propagate into billions/paramDisplay,
// which would otherwise make sizeBytes compute an Inf/garbage size.
func TestModelMetadataNonFiniteGuard(t *testing.T) {
	t.Parallel()

	// A run of digits long enough that ParseFloat overflows to +Inf.
	huge := strings.Repeat("9", 400) + "b"
	_, paramDisplay, billions, _ := modelMetadata(huge)
	if paramDisplay != "" {
		t.Errorf("paramDisplay = %q, want empty on non-finite parse", paramDisplay)
	}
	if billions != 0 {
		t.Errorf("billions = %v, want 0 on non-finite parse", billions)
	}
}

func TestOllamaTagsFormat(t *testing.T) {
	t.Parallel()
	ts := testServer(t, "llama3.2", "phi4")

	resp := getURL(t, ts.URL+"/api/tags")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Models []struct {
			Name    string `json:"name"`
			Model   string `json:"model"`
			Digest  string `json:"digest"`
			Details struct {
				Family string `json:"family"`
			} `json:"details"`
		} `json:"models"`
	}
	decodeJSON(t, resp, &body)

	if len(body.Models) != 2 {
		t.Fatalf("want 2 models, got %d", len(body.Models))
	}
	m := body.Models[0]
	if m.Name == "" {
		t.Error("name field is empty")
	}
	if m.Model == "" {
		t.Error("model field is empty")
	}
	if m.Digest == "" {
		t.Error("digest field is empty")
	}
	if m.Details.Family == "" {
		t.Error("details.family field is empty")
	}
}

func TestOpenAIModelsList(t *testing.T) {
	t.Parallel()
	ts := testServer(t, "gpt-mock")

	resp := getURL(t, ts.URL+"/v1/models")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Object string `json:"object"`
		Data   []struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"data"`
	}
	decodeJSON(t, resp, &body)

	if body.Object != "list" {
		t.Errorf("want object=list, got %q", body.Object)
	}
	if len(body.Data) != 1 {
		t.Fatalf("want 1 model, got %d", len(body.Data))
	}
	if body.Data[0].ID != "gpt-mock" {
		t.Errorf("want id=gpt-mock, got %q", body.Data[0].ID)
	}
	if body.Data[0].Object != "model" {
		t.Errorf("want object=model, got %q", body.Data[0].Object)
	}
}

func TestOpenAINonStreamChat(t *testing.T) {
	t.Parallel()
	ts := testServer(t)

	resp := postJSON(t, ts.URL+"/v1/chat/completions",
		`{"model":"test-model","stream":false,"messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	var body struct {
		Object  string `json:"object"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	decodeJSON(t, resp, &body)

	if body.Object != "chat.completion" {
		t.Errorf("want object=chat.completion, got %q", body.Object)
	}
	if len(body.Choices) == 0 {
		t.Fatal("no choices in response")
	}
	if body.Choices[0].FinishReason != "stop" {
		t.Errorf("want finish_reason=stop, got %q", body.Choices[0].FinishReason)
	}
	if body.Usage.TotalTokens == 0 {
		t.Error("usage.total_tokens is 0")
	}
}

func TestOpenAIStreamChat(t *testing.T) {
	t.Parallel()
	ts := testServer(t)

	resp := postJSON(t, ts.URL+"/v1/chat/completions",
		`{"model":"test-model","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var chunks []string
	var sawDone bool
	var sawContent bool

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if line == "data: [DONE]" {
			sawDone = true
			break
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		chunks = append(chunks, data)

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err == nil {
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				sawContent = true
			}
		}
	}

	if !sawDone {
		t.Error("stream did not end with data: [DONE]")
	}
	if !sawContent {
		t.Error("no chunk with delta content found")
	}
	if len(chunks) == 0 {
		t.Error("no SSE chunks received")
	}
}

func TestOllamaChatStreamDefault(t *testing.T) {
	t.Parallel()
	ts := testServer(t)

	// Ollama defaults to streaming when stream field is absent.
	resp := postJSON(t, ts.URL+"/api/chat",
		`{"model":"test-model","messages":[{"role":"user","content":"hi"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	var lastLine map[string]any
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
