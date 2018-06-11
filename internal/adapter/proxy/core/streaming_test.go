package core

import (
	"context"
	"net/http"
	"testing"

	"github.com/putrkobey/Olla/internal/core/constants"
)

/**
Tests inspired by Scout's streaming detection logic & tests

Keep these in sync so they're similar to avoid regressions
*/

func TestDetectStreamingMode(t *testing.T) {
	tests := []struct {
		name        string
		profile     string
		contentType string
		ctxStream   interface{} // Value for "stream" in context
		want        bool
	}{
		// Profile takes precedence - when ops explicitly configure streaming/buffering
		{
			name:        "explicit streaming profile overrides content type",
			profile:     constants.ConfigurationProxyProfileStreaming,
			contentType: constants.ContentTypeImagePNG,
			want:        true,
		},
		{
			name:        "explicit standard profile overrides content type",
			profile:     constants.ConfigurationProxyProfileStandard,
			contentType: constants.ContentTypeEventStream,
			want:        false,
		},

		// Auto mode recognises known streaming formats
		{
			name:        "auto mode detects SSE streaming",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeEventStream,
			want:        true,
		},
		{
			name:        "auto mode detects NDJSON streaming",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeNDJSON,
			want:        true,
		},
		{
			name:        "auto mode detects streaming JSON",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeStreamJSON,
			want:        true,
		},
		{
			name:        "auto mode detects JSON sequence",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeJSONSeq,
			want:        true,
		},
		{
			name:        "auto mode detects text/plain streaming (some LLMs)",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeTextUTF8,
			want:        true,
		},

		// Binary files get buffered in auto mode to prevent corruption
		{
			name:        "auto mode buffers PNG images",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeImagePNG,
			want:        false,
		},
		{
			name:        "auto mode buffers JPEG images",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeImageJPEG,
			want:        false,
		},
		{
			name:        "auto mode buffers WebP images",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeImageWebP,
			want:        false,
		},
		{
			name:        "auto mode buffers videos",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeVideoMP4,
			want:        false,
		},
		{
			name:        "auto mode buffers audio",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: "audio/mpeg",
			want:        false,
		},
		{
			name:        "auto mode buffers PDFs",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypePDF,
			want:        false,
		},
		{
			name:        "auto mode buffers ZIP files",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeZIP,
			want:        false,
		},
		{
			name:        "auto mode buffers octet-stream",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeOctetStream,
			want:        false,
		},
		{
			name:        "auto mode buffers Excel files",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeExcel,
			want:        false,
		},
		{
			name:        "auto mode buffers Word documents",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeWordDOCX,
			want:        false,
		},
		{
			name:        "auto mode buffers fonts",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: "font/woff2",
			want:        false,
		},
		{
			name:        "auto mode buffers 3D models",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: "model/gltf-binary",
			want:        false,
		},

		// Text formats stream by default - better UX for LLM responses
		{
			name:        "auto mode streams JSON by default",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeJSON,
			want:        true,
		},
		{
			name:        "auto mode streams HTML",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeHTML,
			want:        true,
		},
		{
			name:        "auto mode streams XML",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeXML,
			want:        true,
		},
		{
			name:        "auto mode streams plain text",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeText,
			want:        true,
		},

		// Client preferences from original request context
		{
			name:        "context stream=true forces streaming for JSON",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeJSON,
			ctxStream:   true,
			want:        true,
		},
		{
			name:        "context stream=false doesn't force buffering in auto mode",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeJSON,
			ctxStream:   false,
			want:        true, // Text still streams regardless
		},
		{
			name:        "context stream=true forces streaming even for images in auto mode",
			profile:     constants.ConfigurationProxyProfileAuto,
			contentType: constants.ContentTypeImagePNG,
			ctxStream:   true,
			want:        true,
		},

