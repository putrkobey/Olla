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
