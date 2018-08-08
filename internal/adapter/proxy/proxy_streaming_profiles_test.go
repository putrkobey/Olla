package proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// mockResponseWriter tracks flush calls for testing
type mockResponseWriter struct {
	httptest.ResponseRecorder
	flushCount int
	mu         sync.Mutex
}

func (m *mockResponseWriter) Flush() {
	m.mu.Lock()
	m.flushCount++
	m.mu.Unlock()
}

func (m *mockResponseWriter) getFlushCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.flushCount
}

// TestStreamingProfiles tests that proxy profiles correctly control flushing behavior
func TestStreamingProfiles(t *testing.T) {
	suites := []ProxyTestSuite{
		SherpaTestSuite{},
		OllaTestSuite{},
	}

	for _, suite := range suites {
		t.Run(suite.Name(), func(t *testing.T) {
			testStreamingProfilesForSuite(t, suite)
		})
	}
}

func testStreamingProfilesForSuite(t *testing.T, suite ProxyTestSuite) {
	tests := []struct {
		name           string
		profile        string
		contentType    string
		responseChunks int
		expectFlushes  bool
		description    string
	}{
		// standard profile - should NEVER flush
		{
			name:           "standard_profile_sse_content",
			profile:        constants.ConfigurationProxyProfileStandard,
			contentType:    constants.ContentTypeEventStream,
			responseChunks: 5,
			expectFlushes:  false,
			description:    "standard profile should not flush even for SSE content",
		},
		{
			name:           "standard_profile_json_streaming",
			profile:        constants.ConfigurationProxyProfileStandard,
			contentType:    constants.ContentTypeStreamJSON,
			responseChunks: 5,
			expectFlushes:  false,
			description:    "standard profile should not flush streaming JSON",
		},
		{
			name:           "standard_profile_plain_text",
			profile:        constants.ConfigurationProxyProfileStandard,
			contentType:    constants.ContentTypeText,
			responseChunks: 5,
			expectFlushes:  false,
			description:    "standard profile should not flush plain text",
		},

		// Streaming profile - should ALWAYS flush
		{
			name:           "streaming_profile_binary_content",
			profile:        constants.ConfigurationProxyProfileStreaming,
			contentType:    constants.ContentTypeImagePNG,
			responseChunks: 5,
			expectFlushes:  true,
			description:    "Streaming profile should flush even for binary content",
		},
		{
			name:           "streaming_profile_json",
			profile:        constants.ConfigurationProxyProfileStreaming,
			contentType:    constants.ContentTypeJSON,
			responseChunks: 5,
			expectFlushes:  true,
			description:    "Streaming profile should flush JSON responses",
		},
		{
			name:           "streaming_profile_pdf",
			profile:        constants.ConfigurationProxyProfileStreaming,
			contentType:    constants.ContentTypePDF,
			responseChunks: 5,
			expectFlushes:  true,
			description:    "Streaming profile should flush even PDFs",
		},

		// Auto profile - flush based on content type detection
		{
			name:           "auto_profile_sse_content",
			profile:        constants.ConfigurationProxyProfileAuto,
			contentType:    constants.ContentTypeEventStream,
			responseChunks: 5,
			expectFlushes:  true,
			description:    "Auto profile should flush SSE content",
		},
		{
			name:           "auto_profile_ndjson",
			profile:        constants.ConfigurationProxyProfileAuto,
			contentType:    constants.ContentTypeNDJSON,
			responseChunks: 5,
			expectFlushes:  true,
			description:    "Auto profile should flush NDJSON",
		},
		{
			name:           "auto_profile_binary_image",
			profile:        constants.ConfigurationProxyProfileAuto,
			contentType:    constants.ContentTypeImageJPEG,
			responseChunks: 5,
			expectFlushes:  false,
			description:    "Auto profile should NOT flush binary images",
		},
		{
