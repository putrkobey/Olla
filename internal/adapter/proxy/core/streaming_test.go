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
