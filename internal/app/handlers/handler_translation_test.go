package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/putrkobey/Olla/internal/util"
)

// mockTranslator implements RequestTranslator for testing
type mockTranslator struct {
	name                      string
	transformRequestFunc      func(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error)
	transformResponseFunc     func(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error)
	transformStreamingFunc    func(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error
	writeErrorFunc            func(w http.ResponseWriter, err error, statusCode int)
	countTokensFunc           func(ctx context.Context, r *http.Request) (*translator.TokenCountResponse, error)
	serialiseCountTokensFunc  func(resp *translator.TokenCountResponse) ([]byte, error)
	pathProvider              string
	implementsErrorWriter     bool
	implementsPathProvider    bool
	implementsTokenCounter    bool
	implementsTokenSerializer bool
}

func (m *mockTranslator) Name() string {
	return m.name
}

func (m *mockTranslator) TransformRequest(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
	if m.transformRequestFunc != nil {
		return m.transformRequestFunc(ctx, r)
	}
	return &translator.TransformedRequest{
		OpenAIRequest: map[string]interface{}{
			"model": "test-model",
			"messages": []interface{}{
				map[string]interface{}{
					"role":    "user",
					"content": "test",
				},
			},
		},
		ModelName:   "test-model",
		IsStreaming: false,
	}, nil
}

func (m *mockTranslator) TransformResponse(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error) {
	if m.transformResponseFunc != nil {
		return m.transformResponseFunc(ctx, openaiResp, original)
	}
	return map[string]interface{}{
		"id":      "mock-response-id",
		"content": "mock response",
	}, nil
}

func (m *mockTranslator) TransformStreamingResponse(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error {
	if m.transformStreamingFunc != nil {
		return m.transformStreamingFunc(ctx, openaiStream, w, original)
	}
	w.Header().Set(constants.HeaderContentType, "text/event-stream")
	_, err := io.Copy(w, openaiStream)
	return err
}

func (m *mockTranslator) GetAPIPath() string {
	if m.implementsPathProvider {
		return m.pathProvider
	}
	panic("GetAPIPath called on translator that doesn't implement PathProvider")
}

func (m *mockTranslator) WriteError(w http.ResponseWriter, err error, statusCode int) {
	if m.implementsErrorWriter && m.writeErrorFunc != nil {
		m.writeErrorFunc(w, err, statusCode)
		return
	}
	panic("WriteError called on translator that doesn't implement ErrorWriter")
}

func (m *mockTranslator) CountTokens(ctx context.Context, r *http.Request) (*translator.TokenCountResponse, error) {
	if m.implementsTokenCounter && m.countTokensFunc != nil {
		return m.countTokensFunc(ctx, r)
	}
	panic("CountTokens called on translator that doesn't implement TokenCounter")
}

func (m *mockTranslator) SerialiseCountTokens(resp *translator.TokenCountResponse) ([]byte, error) {
	if m.implementsTokenSerializer && m.serialiseCountTokensFunc != nil {
		return m.serialiseCountTokensFunc(resp)
	}
	panic("SerialiseCountTokens called on translator that doesn't implement TokenCountSerializer")
}

// mockProxyService implements ProxyService for testing
type mockProxyService struct {
	proxyFunc func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, logger logger.StyledLogger) error
}

func (m *mockProxyService) ProxyRequestToEndpoints(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	endpoints []*domain.Endpoint,
	stats *ports.RequestStats,
	logger logger.StyledLogger,
) error {
	if m.proxyFunc != nil {
		return m.proxyFunc(ctx, w, r, endpoints, stats, logger)
	}
	response := map[string]interface{}{
		"id":      "chatcmpl-123",
		"object":  "chat.completion",
		"created": 1677652288,
		"model":   "test-model",
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Hello! How can I help you?",
				},
				"finish_reason": "stop",
			},
		},
	}

	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.Header().Set(constants.HeaderXOllaRequestID, "test-request-id")
	w.Header().Set(constants.HeaderXOllaEndpoint, "test-endpoint")
	w.Header().Set(constants.HeaderXOllaBackendType, "openai")
	w.Header().Set(constants.HeaderXOllaModel, "test-model")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(response)
}

// TestTranslationHandler_PlainTextBackendError verifies that when a backend returns
// a non-JSON 4xx/5xx body (e.g. a plain-text 429 from a rate-limiter), the handler
// preserves the upstream status code and emits a structured Anthropic error rather
// than surfacing a misleading "failed to parse OpenAI response" 502.
func TestTranslationHandler_PlainTextBackendError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		backendStatus  int
		backendBody    string
		backendCT      string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "plain_text_429",
			backendStatus:  http.StatusTooManyRequests,
			backendBody:    "Rate limit exceeded. Please try again later.",
			backendCT:      "text/plain",
			expectedStatus: http.StatusTooManyRequests,
			expectedType:   "rate_limit_error",
		},
		{
			name:           "html_503",
			backendStatus:  http.StatusServiceUnavailable,
			backendBody:    "<html><body>Service Unavailable</body></html>",
			backendCT:      "text/html",
			expectedStatus: http.StatusServiceUnavailable,
			expectedType:   "overloaded_error", // mock writeErrorFunc maps 503 → overloaded_error
		},
		{
			name:          "json_400_preserved",
			backendStatus: http.StatusBadRequest,
			backendBody:   `{"error":{"message":"Invalid temperature value","type":"invalid_request_error"}}`,
			backendCT:     "application/json",
			// Existing behaviour must be preserved: message extracted from JSON.
			expectedStatus: http.StatusBadRequest,
			expectedType:   "invalid_request_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockLogger := &mockStyledLogger{}

			trans := &mockTranslator{
				name:                  "anthropic",
				implementsErrorWriter: true,
				writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
					errorType := "api_error"
					switch statusCode {
					case http.StatusBadRequest:
						errorType = "invalid_request_error"
					case http.StatusTooManyRequests:
						errorType = "rate_limit_error"
					case http.StatusServiceUnavailable:
						errorType = "overloaded_error"
					}
					w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
					w.WriteHeader(statusCode)
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"type": "error",
						"error": map[string]interface{}{
							"type":    errorType,
							"message": err.Error(),
						},
					})
				},
			}

			proxyService := &mockProxyService{
				proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, rlog logger.StyledLogger) error {
					w.Header().Set(constants.HeaderContentType, tt.backendCT)
					w.WriteHeader(tt.backendStatus)
					_, err := w.Write([]byte(tt.backendBody))
					return err
				},
			}

			app := &Application{
				logger:           mockLogger,
				proxyService:     proxyService,
				statsCollector:   &mockStatsCollector{},
				repository:       &mockEndpointRepository{},
				inspectorChain:   inspector.NewChain(mockLogger),
				profileFactory:   &mockProfileFactory{},
				discoveryService: &mockDiscoveryServiceForTranslation{},
				Config:           &config.Config{},
			}

			reqBody, _ := json.Marshal(map[string]interface{}{
				"model":      "test-model",
				"max_tokens": 100,
				"messages":   []map[string]interface{}{{"role": "user", "content": "hello"}},
			})

			req := httptest.NewRequest("POST", "/olla/anthropic/v1/messages", bytes.NewReader(reqBody))
			req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)
			rec := httptest.NewRecorder()

			handler := app.translationHandler(trans)
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code,
				"upstream status code must be preserved, not flattened to 502")

			var errBody map[string]interface{}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody),
				"error body must be valid JSON")
			assert.Equal(t, "error", errBody["type"])

			errObj, ok := errBody["error"].(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, tt.expectedType, errObj["type"],
				"error type must match the expected Anthropic error type for this status")
		})
	}
}

func TestTranslationHandler_NonStreaming(t *testing.T) {
	mockLogger := &mockStyledLogger{}
	trans := &mockTranslator{
		name:                  "test-translator",
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
		},
	}

	app := &Application{
		logger:           mockLogger,
		proxyService:     &mockProxyService{},
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{},
		inspectorChain:   inspector.NewChain(mockLogger),
		profileFactory:   &mockProfileFactory{},
		discoveryService: &mockDiscoveryServiceForTranslation{},
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	reqBody := map[string]interface{}{
		"model": "test-model",
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello",
			},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set(constants.HeaderContentType, constants.ContentTypeJSON)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, constants.ContentTypeJSON, rec.Header().Get(constants.HeaderContentType))

	// Verify X-Olla-* headers are preserved
	assert.NotEmpty(t, rec.Header().Get(constants.HeaderXOllaRequestID))
	assert.NotEmpty(t, rec.Header().Get(constants.HeaderXOllaEndpoint))

	// Verify response was transformed
	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "mock-response-id", response["id"])
}

func TestTranslationHandler_Streaming(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	streamingTrans := &mockTranslator{
		name:                  "streaming-translator",
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
		},
		transformRequestFunc: func(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
			return &translator.TransformedRequest{
				OpenAIRequest: map[string]interface{}{
					"model":  "test-model",
					"stream": true,
					"messages": []interface{}{
						map[string]interface{}{
							"role":    "user",
							"content": "test",
						},
					},
				},
				ModelName:   "test-model",
				IsStreaming: true,
			}, nil
		},
		transformStreamingFunc: func(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error {
			// Read from stream and write transformed output
			w.Header().Set(constants.HeaderContentType, "text/event-stream")
			data, _ := io.ReadAll(openaiStream)
			_, err := w.Write([]byte("data: transformed-" + string(data) + "\n\n"))
			return err
		},
	}

	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, logger logger.StyledLogger) error {
			w.Header().Set(constants.HeaderContentType, "text/event-stream")
			w.Header().Set(constants.HeaderXOllaRequestID, "streaming-test-id")
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
			return err
		},
	}

	app := &Application{
		logger:           mockLogger,
		proxyService:     proxyService,
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{},
		inspectorChain:   inspector.NewChain(mockLogger),
		profileFactory:   &mockProfileFactory{},
		converterFactory: nil,
		discoveryService: &mockDiscoveryServiceForTranslation{},
		Config:           &config.Config{},
	}

	handler := app.translationHandler(streamingTrans)

	reqBody := map[string]interface{}{
		"model":  "test-model",
		"stream": true,
		"messages": []interface{}{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello",
			},
		},
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/event-stream", rec.Header().Get(constants.HeaderContentType))

	// Verify streaming response was transformed
	assert.Contains(t, rec.Body.String(), "transformed-")
}

func TestTranslationHandler_TransformRequestError(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslator{
		name: "error-translator",
		transformRequestFunc: func(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
			return nil, errors.New("invalid request format")
		},
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"type": "error",
				"error": map[string]interface{}{
					"type":    "invalid_request_error",
					"message": err.Error(),
				},
			})
		},
	}

	app := &Application{
		logger:           mockLogger,
		proxyService:     &mockProxyService{},
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{},
		inspectorChain:   inspector.NewChain(mockLogger),
		profileFactory:   &mockProfileFactory{},
		discoveryService: &mockDiscoveryServiceForTranslation{},
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("{}")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "error", response["type"])
}

func TestTranslationHandler_NoHealthyEndpoints(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslator{
		name:                  "test-translator",
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message": err.Error(),
					"type":    "service_unavailable",
				},
			})
		},
	}

	noEndpointsDiscovery := &mockDiscoveryServiceWithFunc{
		getHealthyEndpointsFunc: func(ctx context.Context) ([]*domain.Endpoint, error) {
			return nil, errors.New("no healthy endpoints available")
		},
	}

	app := &Application{
		logger:           mockLogger,
		proxyService:     &mockProxyService{},
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{},
		inspectorChain:   inspector.NewChain(mockLogger),
		profileFactory:   &mockProfileFactory{},
		converterFactory: nil,
		discoveryService: noEndpointsDiscovery,
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	reqBody := map[string]interface{}{"model": "test-model"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestTranslationHandler_HeaderPreservation(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslator{
		name: "test-translator",
	}

	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, logger logger.StyledLogger) error {
			// Set all X-Olla-* headers
			w.Header().Set(constants.HeaderXOllaRequestID, "test-request-id")
			w.Header().Set(constants.HeaderXOllaEndpoint, "test-endpoint")
			w.Header().Set(constants.HeaderXOllaBackendType, "openai")
			w.Header().Set(constants.HeaderXOllaModel, "test-model")
			w.Header().Set(constants.HeaderXOllaResponseTime, "123ms")
			w.Header().Set(constants.HeaderXOllaRoutingStrategy, "priority")
			w.Header().Set(constants.HeaderXOllaRoutingDecision, "selected")
			w.Header().Set(constants.HeaderXOllaRoutingReason, "health check passed")

			response := map[string]interface{}{
				"id":      "test-id",
				"choices": []interface{}{},
			}
			return json.NewEncoder(w).Encode(response)
		},
	}

	app := &Application{
		logger:           mockLogger,
		proxyService:     proxyService,
		statsCollector:   &mockStatsCollector{},
		repository:       &mockEndpointRepository{},
		inspectorChain:   inspector.NewChain(mockLogger),
		profileFactory:   &mockProfileFactory{},
		converterFactory: nil,
		discoveryService: &mockDiscoveryServiceForTranslation{},
		Config:           &config.Config{},
	}

	handler := app.translationHandler(trans)

	reqBody := map[string]interface{}{"model": "test-model"}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify all X-Olla-* headers are preserved
	assert.Equal(t, "test-request-id", rec.Header().Get(constants.HeaderXOllaRequestID))
	assert.Equal(t, "test-endpoint", rec.Header().Get(constants.HeaderXOllaEndpoint))
	assert.Equal(t, "openai", rec.Header().Get(constants.HeaderXOllaBackendType))
	assert.Equal(t, "test-model", rec.Header().Get(constants.HeaderXOllaModel))
	assert.Equal(t, "123ms", rec.Header().Get(constants.HeaderXOllaResponseTime))
	assert.Equal(t, "priority", rec.Header().Get(constants.HeaderXOllaRoutingStrategy))
	assert.Equal(t, "selected", rec.Header().Get(constants.HeaderXOllaRoutingDecision))
	assert.Equal(t, "health check passed", rec.Header().Get(constants.HeaderXOllaRoutingReason))
}

func TestWriteTranslatorError_WithErrorWriter(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslator{
		name:                  "test-translator",
		implementsErrorWriter: true,
		writeErrorFunc: func(w http.ResponseWriter, err error, statusCode int) {
			w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
			w.WriteHeader(statusCode)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"custom_error": err.Error(),
			})
		},
	}

	app := &Application{
		logger: mockLogger,
		Config: &config.Config{},
	}

	rec := httptest.NewRecorder()
	pr := &proxyRequest{
		requestLogger: mockLogger,
	}

	app.writeTranslatorError(rec, trans, pr, errors.New("test error"), http.StatusBadRequest)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test error", response["custom_error"])
}

func TestWriteTranslatorError_WithoutErrorWriter(t *testing.T) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslatorWithoutErrorWriter{
		name: "test-translator",
	}

	app := &Application{
		logger: mockLogger,
		Config: &config.Config{},
	}

	rec := httptest.NewRecorder()
	pr := &proxyRequest{
		requestLogger: mockLogger,
	}

	app.writeTranslatorError(rec, trans, pr, errors.New("test error"), http.StatusInternalServerError)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	var response map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	errorObj, ok := response["error"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test error", errorObj["message"])
	assert.Equal(t, "translation_error", errorObj["type"])
}

func (m *mockProxyService) GetStats(ctx context.Context) (ports.ProxyStats, error) {
	return ports.ProxyStats{}, nil
}
func (m *mockProxyService) UpdateConfig(configuration ports.ProxyConfiguration) {}
func (m *mockProxyService) ProxyRequest(ctx context.Context, w http.ResponseWriter, r *http.Request, stats *ports.RequestStats, rlog logger.StyledLogger) error {
	return nil
}

type mockStatsCollector struct{}

func (m *mockStatsCollector) RecordRequest(endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockStatsCollector) RecordConnection(endpoint *domain.Endpoint, delta int) {}
func (m *mockStatsCollector) RecordSecurityViolation(violation ports.SecurityViolation) {
}
func (m *mockStatsCollector) RecordDiscovery(endpoint *domain.Endpoint, success bool, latency time.Duration) {
}
func (m *mockStatsCollector) RecordModelRequest(model string, endpoint *domain.Endpoint, status string, latency time.Duration, bytes int64) {
}
func (m *mockStatsCollector) RecordModelError(model string, endpoint *domain.Endpoint, errorType string) {
}
func (m *mockStatsCollector) GetModelStats() map[string]ports.ModelStats { return nil }
func (m *mockStatsCollector) GetModelEndpointStats() map[string]map[string]ports.EndpointModelStats {
	return nil
}
func (m *mockStatsCollector) RecordTranslatorRequest(event ports.TranslatorRequestEvent) {}
func (m *mockStatsCollector) GetTranslatorStats() map[string]ports.TranslatorStats {
	return nil
}
func (m *mockStatsCollector) GetProxyStats() ports.ProxyStats                  { return ports.ProxyStats{} }
func (m *mockStatsCollector) GetEndpointStats() map[string]ports.EndpointStats { return nil }
func (m *mockStatsCollector) GetSecurityStats() ports.SecurityStats            { return ports.SecurityStats{} }
func (m *mockStatsCollector) GetConnectionStats() map[string]int64             { return nil }
func (m *mockStatsCollector) GetConnectionCount(endpoint string) int64         { return 0 }

type mockEndpointRepository struct {
	getEndpointsFunc func() []*domain.Endpoint
}

func (m *mockEndpointRepository) GetEndpoints() []*domain.Endpoint {
	if m.getEndpointsFunc != nil {
		return m.getEndpointsFunc()
	}
	u, _ := url.Parse("http://localhost:8080")
	return []*domain.Endpoint{
		{
			Name:      "test-endpoint",
			URL:       u,
			URLString: "http://localhost:8080",
			Type:      "openai",
			Status:    domain.StatusHealthy,
		},
	}
}

func (m *mockEndpointRepository) GetEndpoint(name string) (*domain.Endpoint, error) {
	return nil, nil
}

func (m *mockEndpointRepository) AddEndpoint(endpoint *domain.Endpoint) error {
	return nil
}

func (m *mockEndpointRepository) RemoveEndpoint(name string) error {
	return nil
}

func (m *mockEndpointRepository) Exists(ctx context.Context, endpointURL *url.URL) bool {
	return false
}

func (m *mockEndpointRepository) GetAll(ctx context.Context) ([]*domain.Endpoint, error) {
	if m.getEndpointsFunc != nil {
		return m.getEndpointsFunc(), nil
	}
	u, _ := url.Parse("http://localhost:8080")
	return []*domain.Endpoint{
		{
			Name:      "test-endpoint",
			URL:       u,
			URLString: "http://localhost:8080",
			Type:      "openai",
			Status:    domain.StatusHealthy,
		},
	}, nil
}

func (m *mockEndpointRepository) GetRoutable(ctx context.Context) ([]*domain.Endpoint, error) {
	return m.GetHealthy(ctx)
}

func (m *mockEndpointRepository) UpdateEndpoint(ctx context.Context, endpoint *domain.Endpoint) error {
	return nil
}

func (m *mockEndpointRepository) GetHealthy(ctx context.Context) ([]*domain.Endpoint, error) {
	if m.getEndpointsFunc != nil {
		return m.getEndpointsFunc(), nil
	}
	u, _ := url.Parse("http://localhost:8080")
	return []*domain.Endpoint{
		{
			Name:      "test-endpoint",
			URL:       u,
			URLString: "http://localhost:8080",
			Type:      "openai",
			Status:    domain.StatusHealthy,
		},
	}, nil
}
