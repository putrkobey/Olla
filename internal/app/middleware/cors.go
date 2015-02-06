package middleware

import (
	"github.com/rs/cors"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// DefaultCORSExposedHeaders is the set of X-Olla-* response headers exposed to
// browser clients when the caller has not configured explicit ExposedHeaders.
//
// By default browsers block non-simple response headers from cross-origin reads.
// Olla's routing, model, and sticky-session metadata all live in X-Olla-* headers,
// so browser clients (OpenWebUI, custom dashboards) cannot inspect them without
// this exposure list. We expose the full set rather than a subset to avoid
// surprising omissions when new headers are added to the proxy output.
var DefaultCORSExposedHeaders = []string{
	constants.HeaderXOllaRequestID,
	constants.HeaderXOllaEndpoint,
	constants.HeaderXOllaBackendType,
	constants.HeaderXOllaModel,
	constants.HeaderXOllaResponseTime,
	constants.HeaderXOllaRoutingStrategy,
	constants.HeaderXOllaRoutingDecision,
	constants.HeaderXOllaRoutingReason,
