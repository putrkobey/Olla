package inspector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

func TestBodyInspector_Inspect(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}

	tests := []struct {
		name           string
		body           string
		contentType    string
		expectedModel  string
		skipInspection bool
	}{
		{
			name:          "OpenAI format",
			body:          `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			contentType:   "application/json",
			expectedModel: "gpt-4",
		},
		{
			name:          "Ollama format with tag",
			body:          `{"model": "llama3.1:8b", "prompt": "Hello"}`,
			contentType:   "application/json",
			expectedModel: "llama3.1:8b",
		},
		{
			name:          "LM Studio format",
			body:          `{"model": "meta-llama-3.1-8b-instruct", "messages": []}`,
			contentType:   "application/json",
			expectedModel: "meta-llama-3.1-8b-instruct",
		},
		{
			name:          "Model with latest tag",
			body:          `{"model": "codellama:latest"}`,
			contentType:   "application/json",
			expectedModel: "codellama:latest",
		},
		{
			name:          "Model with uppercase",
			body:          `{"model": "GPT-4-Turbo"}`,
			contentType:   "application/json",
			expectedModel: "gpt-4-turbo",
		},
		{
			name:          "Model with whitespace",
			body:          `{"model": "  llama3.1:8b  "}`,
			contentType:   "application/json",
			expectedModel: "llama3.1:8b",
		},
		{
			name:          "Empty model field",
			body:          `{"model": "", "messages": []}`,
			contentType:   "application/json",
			expectedModel: "",
		},
		{
			name:          "No model field",
			body:          `{"messages": [{"role": "user", "content": "Hello"}]}`,
			contentType:   "application/json",
			expectedModel: "",
		},
		{
			// the body decides, not the header: fetch() and curl -d send JSON as text/plain
			// and form-urlencoded
			name:          "JSON body with text/plain content type",
			body:          `{"model": "gpt-4"}`,
			contentType:   "text/plain",
			expectedModel: "gpt-4",
		},
		{
			name:          "Invalid JSON",
			body:          `{invalid json}`,
			contentType:   "application/json",
			expectedModel: "",
		},
		{
			name:          "Case insensitive model key",
			body:          `{"Model": "gpt-4"}`,
			contentType:   "application/json",
			expectedModel: "gpt-4",
		},
		{
			name:          "Nested object",
			body:          `{"request": {"model": "gpt-4"}, "model": "llama3"}`,
			contentType:   "application/json",
			expectedModel: "llama3", // Should get top-level model
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inspector, err := NewBodyInspector(styledLog)
			if err != nil {
				t.Fatalf("Failed to create body inspector: %v", err)
			}

			// Create request with body
			req := &http.Request{
				Body:   io.NopCloser(strings.NewReader(tt.body)),
				Header: make(http.Header),
			}
			req.Header.Set("Content-Type", tt.contentType)
			req.ContentLength = int64(len(tt.body))

			// Create profile
			profile := domain.NewRequestProfile("/v1/chat/completions")

			// Inspect
			err = inspector.Inspect(ctx, req, profile)
			require.NoError(t, err)

			// Verify model extraction
			assert.Equal(t, tt.expectedModel, profile.ModelName)

			// Verify body is still readable
			if !tt.skipInspection && tt.body != "" {
				bodyBytes, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, tt.body, string(bodyBytes))
			}
		})
	}
}

func TestBodyInspector_LargeBody(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}
	inspector, err := NewBodyInspector(styledLog)
	if err != nil {
		t.Fatalf("Failed to create body inspector: %v", err)
	}

	// Build a large JSON body that mimics a vision request: the model field is at the very
	// start (well within modelScanSize), while the bulk of the body is a large base64 image.
	imagePayload := strings.Repeat("A", MaxBodySize+1000)
	largeBody := `{"model":"vision-model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` + imagePayload + `"}}]}]}`

	req := &http.Request{
		Body:          io.NopCloser(strings.NewReader(largeBody)),
		Header:        make(http.Header),
		ContentLength: int64(len(largeBody)),
	}
	req.Header.Set("Content-Type", "application/json")

	profile := domain.NewRequestProfile("/v1/chat/completions")

	// Large body: model name must still be extracted from the prefix scan so routing works.
	// Capability detection is intentionally skipped (no full parse) for large requests.
	err = inspector.Inspect(ctx, req, profile)
	assert.NoError(t, err)
	assert.Equal(t, "vision-model", profile.ModelName, "model name must be extracted even from large vision requests")
	assert.Nil(t, profile.ModelCapabilities, "capability detection must be skipped for large requests")

	// The full body must be restored intact for downstream proxy handlers.
	restored, readErr := io.ReadAll(req.Body)
	assert.NoError(t, readErr)
	assert.Equal(t, largeBody, string(restored), "full body must be restored after prefix scan")
}

// TestBodyInspector_LargeBodyModelAfterMessages verifies that model extraction works
// correctly when the "model" field appears after a large "messages" array (containing
// base64 image payloads) in a request body exceeding MaxBodySize.
// This is the field-order case that the fixed 64 KB prefix approach would silently miss.
func TestBodyInspector_LargeBodyModelAfterMessages(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}
	inspector, err := NewBodyInspector(styledLog)
	require.NoError(t, err)

	// Build a large JSON body where "messages" (with a base64 image payload) appears
	// before "model". The model field is beyond the first 64 KB of the body, which
	// would have caused extraction to fail with the old fixed-prefix approach.
	imagePayload := strings.Repeat("B", MaxBodySize+1000)
	largeBody := `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,` +
		imagePayload + `"}}]}],"model":"vision-model-late"}`

	req := &http.Request{
		Body:          io.NopCloser(strings.NewReader(largeBody)),
		Header:        make(http.Header),
		ContentLength: int64(len(largeBody)),
	}
	req.Header.Set("Content-Type", "application/json")

	profile := domain.NewRequestProfile("/v1/chat/completions")

	err = inspector.Inspect(ctx, req, profile)
	assert.NoError(t, err)
	assert.Equal(t, "vision-model-late", profile.ModelName,
		"model name must be extracted even when 'model' field appears after large 'messages'")

	// The full body must be restored intact for downstream proxy handlers.
	restored, readErr := io.ReadAll(req.Body)
	assert.NoError(t, readErr)
	assert.Equal(t, largeBody, string(restored), "full body must be restored after incremental scan")
}

func TestBodyInspector_NoBody(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}
	inspector, err := NewBodyInspector(styledLog)
	if err != nil {
		t.Fatalf("Failed to create body inspector: %v", err)
	}

	tests := []struct {
		name string
		req  *http.Request
	}{
		{
			name: "nil body",
			req: &http.Request{
				Body:   nil,
				Header: make(http.Header),
			},
		},
		{
			name: "zero content length",
			req: &http.Request{
				Body:          io.NopCloser(strings.NewReader("")),
				Header:        make(http.Header),
				ContentLength: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.req.Header.Set("Content-Type", "application/json")
			profile := domain.NewRequestProfile("/v1/chat/completions")

			err = inspector.Inspect(ctx, tt.req, profile)
			assert.NoError(t, err)
			assert.Empty(t, profile.ModelName)
		})
	}
}

func TestBodyInspector_BufferPoolReuse(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}
	inspector, err := NewBodyInspector(styledLog)
	if err != nil {
		t.Fatalf("Failed to create body inspector: %v", err)
	}

	// Run multiple inspections to test buffer pool reuse
	for range 10 {
		body := `{"model": "test-model"}`
		req := &http.Request{
			Body:          io.NopCloser(strings.NewReader(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		req.Header.Set("Content-Type", "application/json")

		profile := domain.NewRequestProfile("/v1/chat/completions")

		err = inspector.Inspect(ctx, req, profile)
		assert.NoError(t, err)
		assert.Equal(t, "test-model", profile.ModelName)
	}
}

func TestBodyInspector_CapabilityDetection(t *testing.T) {
	ctx := context.Background()
	logCfg := &logger.Config{Level: "debug", PrettyLogs: false}
	log, _, err := logger.New(logCfg)
	require.NoError(t, err)
	styledLog := &mockStyledLogger{underlying: log}

	tests := []struct {
		name                   string
		body                   string
		expectedVision         bool
		expectedFunctions      bool
		expectedStreaming      bool
		expectedEmbeddings     bool
		expectedCodeGeneration bool
		expectedChatCompletion bool
		expectedTextGeneration bool
	}{
		{
			name:                   "Simple chat request",
			body:                   `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hello"}]}`,
			expectedVision:         false,
			expectedFunctions:      false,
			expectedStreaming:      true, // Default
			expectedEmbeddings:     false,
			expectedCodeGeneration: false,
			expectedChatCompletion: true,
			expectedTextGeneration: true,
		},
		{
			name: "Vision request with image_url",
			body: `{
				"model": "gpt-4-vision",
				"messages": [{
					"role": "user",
					"content": [{
						"type": "text",
						"text": "What's in this image?"
					}, {
						"type": "image_url",
						"image_url": {"url": "https://example.com/image.jpg"}
					}]
				}]
			}`,
			expectedVision:         true,
			expectedFunctions:      false,
			expectedStreaming:      true,
			expectedEmbeddings:     false,
			expectedCodeGeneration: false,
			expectedChatCompletion: true,
			expectedTextGeneration: true,
		},
		{
			name: "Vision request with base64 image",
			body: `{
				"model": "llava",
				"messages": [{
					"role": "user",
					"content": [{
						"type": "text",
						"text": "data:image/jpeg;base64,/9j/4AAQ..."
					}]
				}]
			}`,
			expectedVision:         true,
			expectedFunctions:      false,
			expectedStreaming:      true,
			expectedEmbeddings:     false,
			expectedCodeGeneration: false,
			expectedChatCompletion: true,
			expectedTextGeneration: true,
		},
		{
			name: "Function calling request with tools",
			body: `{
				"model": "gpt-4-turbo",
				"messages": [{"role": "user", "content": "Get the weather"}],
				"tools": [{
					"type": "function",
					"function": {
						"name": "get_weather",
						"description": "Get weather for a location"
					}
				}]
			}`,
			expectedVision:         false,
			expectedFunctions:      true,
			expectedStreaming:      true,
			expectedEmbeddings:     false,
			expectedCodeGeneration: false,
			expectedChatCompletion: true,
			expectedTextGeneration: true,
		},
		{
			name: "Function calling with functions array",
			body: `{
				"model": "gpt-3.5-turbo",
				"messages": [{"role": "user", "content": "Calculate something"}],
				"functions": [{
					"name": "calculate",
					"description": "Perform calculations"
				}]
			}`,
			expectedVision:         false,
			expectedFunctions:      true,
			expectedStreaming:      true,
			expectedEmbeddings:     false,
			expectedCodeGeneration: false,
			expectedChatCompletion: true,
			expectedTextGeneration: true,
		},
