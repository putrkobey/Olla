package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// BenchmarkTranslationHandler_NonStreamingOverhead measures the overhead
// introduced by the translation layer for non-streaming requests
// Target: <5ms overhead as per specification
func BenchmarkTranslationHandler_NonStreamingOverhead(b *testing.B) {
	mockLogger := &mockStyledLogger{}

	trans := &mockTranslator{
		name: "bench-translator",
		transformRequestFunc: func(ctx context.Context, r *http.Request) (*translator.TransformedRequest, error) {
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
		},
		transformResponseFunc: func(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error) {
			return map[string]interface{}{
				"id":      "bench-response",
				"content": "benchmark response",
			}, nil
		},
	}

	proxyService := &mockProxyService{
		proxyFunc: func(ctx context.Context, w http.ResponseWriter, r *http.Request, endpoints []*domain.Endpoint, stats *ports.RequestStats, logger logger.StyledLogger) error {
			response := map[string]interface{}{
				"id":      "chatcmpl-bench",
				"object":  "chat.completion",
				"created": 1677652288,
				"model":   "test-model",
				"choices": []interface{}{
					map[string]interface{}{
						"index": 0,
						"message": map[string]interface{}{
							"role":    "assistant",
							"content": "Hello!",
