package core

import (
	"context"
	"net/http"
	"strings"

	"github.com/putrkobey/Olla/internal/core/constants"
)

// AutoDetectStreamingMode works out whether to stream or buffer a response
// LLMs benefit from streaming (users see tokens as they're generated),
// but binary files need buffering to avoid corruption and stalls
func AutoDetectStreamingMode(ctx context.Context, resp *http.Response, profile string) bool {
	// Force either if configured
	if profile == constants.ConfigurationProxyProfileStandard {
		return false
	}
	if profile == constants.ConfigurationProxyProfileStreaming {
		return true
	}

	// Auto mode - make an intelligent choice based on response content
	contentType := strings.ToLower(resp.Header.Get(constants.HeaderContentType))

	// if we know the streaming format, get streamed immediately
	if isStreamingContentType(contentType) {
		return true
	}

	// respect client preferences from the original request
	if streamVal := ctx.Value(constants.ContextKeyStream); streamVal != nil {
		if stream, ok := streamVal.(bool); ok && stream {
			return true
		}
	}

	// binary content needs buffering to ensure intact delivery
	if isBinaryContentType(contentType) {
		return false
	}

	// fallback to streaming for text-based responses otherwise
	return true
}

var streamingTypes = []string{
	constants.ContentTypeEventStream,
	constants.ContentTypeNDJSON,
	constants.ContentTypeStreamJSON,
	constants.ContentTypeJSONSeq,
	constants.ContentTypeTextUTF8, // Common fallback for LLM streaming
}

// isStreamingContentType identifies known streaming formats.
// some APIs seem to be explicitly signaling streaming intent through content-type
// headers and we should respect that
func isStreamingContentType(contentType string) bool {
	for _, st := range streamingTypes {
		if strings.Contains(contentType, st) {
			return true
		}
	}
	return false
