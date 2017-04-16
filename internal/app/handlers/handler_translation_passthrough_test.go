package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// TestTranslationHandler_PassthroughNonStreaming tests end-to-end passthrough for non-streaming requests
func TestTranslationHandler_PassthroughNonStreaming(t *testing.T) {
	// Setup mock backend that accepts Anthropic format
	backendCalled := false
	var receivedBody []byte
	var receivedPath string

	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		receivedPath = r.URL.Path

		// Read body to verify it's unchanged
		body, _ := io.ReadAll(r.Body)
		receivedBody = body

		// Return Anthropic format response
		response := map[string]interface{}{
			"id":      "msg_01XFDUDYJgAACzvnptvVoYEL",
			"type":    "message",
			"role":    "assistant",
			"content": []map[string]interface{}{{"type": "text", "text": "Hello! How can I help you?"}},
			"model":   "claude-3-5-sonnet-20241022",
			"usage": map[string]interface{}{
				"input_tokens":  10,
				"output_tokens": 20,
			},
		}

		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Header().Set(constants.HeaderXOllaEndpoint, "test-backend")
		w.Header().Set(constants.HeaderXOllaBackendType, "vllm")
		w.Header().Set(constants.HeaderXOllaModel, "claude-3-5-sonnet-20241022")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer mockBackend.Close()

	// Parse backend URL
	backendURL, _ := url.Parse(mockBackend.URL)

	// Setup endpoints with Anthropic support
	endpoints := []*domain.Endpoint{
		{
			Name:      "vllm-backend",
			URL:       backendURL,
			URLString: mockBackend.URL,
			Type:      "vllm",
			Status:    domain.StatusHealthy,
		},
	}

	// Create mock profile lookup that indicates Anthropic support
	profileLookup := &mockPassthroughProfileLookup{
		configs: map[string]*domain.AnthropicSupportConfig{
			"vllm": {
				Enabled:      true,
				MessagesPath: "/v1/messages",
			},
		},
	}

	// Create passthrough-capable translator
	trans := &mockPassthroughTranslator{
		name:                  "anthropic",
		implementsErrorWriter: true,
		passthroughEnabled:    true,
		profileLookup:         profileLookup,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		},
	}

	// Create proxy service that forwards to backend
	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, eps []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
			// Forward to backend
			client := &http.Client{Timeout: 5 * time.Second}
			backendReq, err := http.NewRequest(r.Method, eps[0].URLString+r.URL.Path, r.Body)
			if err != nil {
				return err
			}
			backendReq.Header = r.Header.Clone()

			resp, err := client.Do(backendReq)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			// Copy headers
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)

			_, err = io.Copy(w, resp.Body)
			return err
		},
	}

	// Create discovery service that returns our endpoints
	discoveryService := &mockDiscoveryServiceWithEndpoints{
		endpoints: endpoints,
	}

	app := &Application{
		logger:           &mockStyledLogger{},
		proxyService:     proxyService,
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{getEndpointsFunc: func() []*domain.Endpoint { return endpoints }},
		inspectorChain:   inspector.NewChain(&mockStyledLogger{}),
		profileFactory:   &mockProfileFactory{},
		profileLookup:    profileLookup,
		discoveryService: discoveryService,
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	// Create Anthropic request
	anthropicReq := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}
	reqBody, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest("POST", "/olla/anthropic/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Assertions
	assert.True(t, backendCalled, "Backend should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify passthrough mode header
	assert.Equal(t, "passthrough", rec.Header().Get("X-Olla-Mode"), "Should have passthrough mode header")

	// Verify request was passed through unchanged
	var receivedReq map[string]interface{}
	err := json.Unmarshal(receivedBody, &receivedReq)
	require.NoError(t, err)
	assert.Equal(t, "claude-3-5-sonnet-20241022", receivedReq["model"])
	assert.Equal(t, float64(1024), receivedReq["max_tokens"])

	// Verify path
	assert.Equal(t, "/v1/messages", receivedPath)

	// Verify response
	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "message", response["type"])
	assert.Equal(t, "msg_01XFDUDYJgAACzvnptvVoYEL", response["id"])

	// Verify X-Olla headers preserved
	assert.NotEmpty(t, rec.Header().Get(constants.HeaderXOllaEndpoint))
	assert.NotEmpty(t, rec.Header().Get(constants.HeaderXOllaBackendType))
}

// TestTranslationHandler_PassthroughStreaming tests end-to-end passthrough for streaming requests
func TestTranslationHandler_PassthroughStreaming(t *testing.T) {
	backendCalled := false
	var receivedPath string

	// Setup mock backend that returns SSE stream
	mockBackend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		receivedPath = r.URL.Path

		// Return SSE stream in Anthropic format
		w.Header().Set(constants.HeaderContentType, "text/event-stream")
		w.Header().Set(constants.HeaderXOllaEndpoint, "test-backend")
		w.Header().Set(constants.HeaderXOllaBackendType, "vllm")
		w.WriteHeader(http.StatusOK)

		// Write SSE events
		events := []string{
			`event: message_start
data: {"type":"message_start","message":{"id":"msg_123","type":"message","role":"assistant"}}

`,
			`event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

`,
			`event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}

`,
			`event: message_stop
data: {"type":"message_stop"}

`,
		}

		for _, event := range events {
			fmt.Fprint(w, event)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer mockBackend.Close()

	backendURL, _ := url.Parse(mockBackend.URL)

	endpoints := []*domain.Endpoint{
		{
			Name:      "vllm-backend",
			URL:       backendURL,
			URLString: mockBackend.URL,
			Type:      "vllm",
			Status:    domain.StatusHealthy,
		},
	}

	profileLookup := &mockPassthroughProfileLookup{
		configs: map[string]*domain.AnthropicSupportConfig{
			"vllm": {
				Enabled:      true,
				MessagesPath: "/v1/messages",
			},
		},
	}

	trans := &mockPassthroughTranslator{
		name:               "anthropic",
		passthroughEnabled: true,
		profileLookup:      profileLookup,
		transformRequestFunc: func(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
			return &translator.TransformedRequest{
				ModelName:   "claude-3-5-sonnet-20241022",
				IsStreaming: true,
			}, nil
		},
	}

	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, eps []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
			client := &http.Client{Timeout: 5 * time.Second}
			backendReq, err := http.NewRequest(r.Method, eps[0].URLString+r.URL.Path, r.Body)
			if err != nil {
				return err
			}

			resp, err := client.Do(backendReq)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)

			_, err = io.Copy(w, resp.Body)
			return err
		},
	}

	discoveryService := &mockDiscoveryServiceWithEndpoints{
		endpoints: endpoints,
	}

	app := &Application{
		logger:           &mockStyledLogger{},
		proxyService:     proxyService,
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{getEndpointsFunc: func() []*domain.Endpoint { return endpoints }},
		inspectorChain:   inspector.NewChain(&mockStyledLogger{}),
		profileFactory:   &mockProfileFactory{},
		profileLookup:    profileLookup,
		discoveryService: discoveryService,
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	anthropicReq := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"stream":     true,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}
	reqBody, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest("POST", "/olla/anthropic/v1/messages", bytes.NewReader(reqBody))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Assertions
	assert.True(t, backendCalled, "Backend should have been called")
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify passthrough mode header
	assert.Equal(t, "passthrough", rec.Header().Get("X-Olla-Mode"))

	// Verify SSE content type
	assert.Equal(t, "text/event-stream", rec.Header().Get(constants.HeaderContentType))

	// Verify path
	assert.Equal(t, "/v1/messages", receivedPath)

	// Verify SSE events are passed through
	body := rec.Body.String()
	assert.Contains(t, body, "event: message_start")
	assert.Contains(t, body, "event: content_block_delta")
	assert.Contains(t, body, "event: message_delta")
	assert.Contains(t, body, "event: message_stop")

	// Verify X-Olla headers
	assert.NotEmpty(t, rec.Header().Get(constants.HeaderXOllaEndpoint))
}

// TestTranslationHandler_PassthroughWithMultipleEndpoints tests passthrough with load balancing
func TestTranslationHandler_PassthroughWithMultipleEndpoints(t *testing.T) {
	backendsCalled := make(map[string]bool)

	// Setup multiple mock backends
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendsCalled["backend1"] = true
		response := map[string]interface{}{"id": "msg_01", "type": "message", "role": "assistant"}
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Header().Set(constants.HeaderXOllaEndpoint, "vllm-1")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendsCalled["backend2"] = true
		response := map[string]interface{}{"id": "msg_02", "type": "message", "role": "assistant"}
		w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
		w.Header().Set(constants.HeaderXOllaEndpoint, "vllm-2")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer backend2.Close()

	backendURL1, _ := url.Parse(backend1.URL)
	backendURL2, _ := url.Parse(backend2.URL)

	endpoints := []*domain.Endpoint{
		{
			Name:      "vllm-1",
			URL:       backendURL1,
			URLString: backend1.URL,
			Type:      "vllm",
			Status:    domain.StatusHealthy,
		},
		{
			Name:      "vllm-2",
			URL:       backendURL2,
			URLString: backend2.URL,
			Type:      "vllm",
			Status:    domain.StatusHealthy,
		},
	}

	profileLookup := &mockPassthroughProfileLookup{
		configs: map[string]*domain.AnthropicSupportConfig{
			"vllm": {
				Enabled:      true,
				MessagesPath: "/v1/messages",
			},
		},
	}

	trans := &mockPassthroughTranslator{
		name:               "anthropic",
		passthroughEnabled: true,
		profileLookup:      profileLookup,
	}

	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, eps []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
			// Use first endpoint (simulating load balancer selection)
			client := &http.Client{Timeout: 5 * time.Second}
			backendReq, err := http.NewRequest(r.Method, eps[0].URLString+r.URL.Path, r.Body)
			if err != nil {
				return err
			}

			resp, err := client.Do(backendReq)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			_, err = io.Copy(w, resp.Body)
			return err
		},
	}

	discoveryService := &mockDiscoveryServiceWithEndpoints{
		endpoints: endpoints,
	}

	app := &Application{
		logger:           &mockStyledLogger{},
		proxyService:     proxyService,
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{getEndpointsFunc: func() []*domain.Endpoint { return endpoints }},
		inspectorChain:   inspector.NewChain(&mockStyledLogger{}),
		profileFactory:   &mockProfileFactory{},
		profileLookup:    profileLookup,
		discoveryService: discoveryService,
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	anthropicReq := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 1024,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
	}
	reqBody, _ := json.Marshal(anthropicReq)

	req := httptest.NewRequest("POST", "/olla/anthropic/v1/messages", bytes.NewReader(reqBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify one backend was called
	assert.Positive(t, len(backendsCalled), "At least one backend should be called")

	// Verify passthrough mode
	assert.Equal(t, "passthrough", rec.Header().Get("X-Olla-Mode"))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestTranslationHandler_PassthroughUsed_MixedEndpoints tests that passthrough is used for the
// capable subset when mixed backends are present (vllm supports Anthropic, ollama does not).
// The handler filters to passthrough-capable endpoints before calling CanPassthrough, so the
// ollama backend is excluded and the vllm backend receives the request directly.
func TestTranslationHandler_PassthroughUsed_MixedEndpoints(t *testing.T) {
	passthroughEndpointsReceived := []*domain.Endpoint{}

	endpoints := []*domain.Endpoint{
		{Name: "vllm-1", Type: "vllm", Status: domain.StatusHealthy},
		{Name: "ollama-1", Type: "ollama", Status: domain.StatusHealthy}, // No Anthropic support
	}
