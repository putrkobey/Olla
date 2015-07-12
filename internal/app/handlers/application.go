package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/balancer"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/adapter/translator"
	"github.com/putrkobey/Olla/internal/adapter/translator/anthropic"
	"github.com/putrkobey/Olla/internal/app/middleware"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
)

// SecurityAdapters provides middleware for security chain
type SecurityAdapters struct {
	securityChain    *ports.SecurityChain
	securityAdapters *security.Adapters // nil when security is not configured
	logger           logger.StyledLogger
	throttles        *logger.ThrottleSet // nil in tests; collects the request-log throttles

	// requestLog is built once and shared by every route: the route registry asks for the
	// chain per route, and a middleware built per route would hold its own rejection
	// throttle, so a client would get one sampled line per route per window rather than one.
	requestLog     func(http.Handler) http.Handler
	requestLogOnce sync.Once
}

func (s *SecurityAdapters) loggingMiddleware() func(http.Handler) http.Handler {
	s.requestLogOnce.Do(func() {
		s.requestLog = middleware.NewCombinedLoggingMiddleware(s.logger, s.throttles)
	})
	return s.requestLog
}

// CreateChainMiddleware creates middleware that applies the full security chain with enhanced logging.
// When concrete security adapters are available (production path), we delegate to them so that
// per-validator status codes (429 rate-limit, 413 body-too-large) are preserved. The abstract
// securityChain path is kept as a fallback for test contexts where only the chain is wired.
func (s *SecurityAdapters) CreateChainMiddleware(reject security.RejectFunc) func(http.Handler) http.Handler {
	return s.chain(reject, func(a *security.Adapters) func(http.Handler) http.Handler {
		return a.CreateChainMiddleware(reject)
	})
}

// CreateTranslatorChainMiddleware is CreateChainMiddleware for translator routes, which buffer
// their bodies and so take the buffering size variant. Rejections are written through reject
// in the translator's own error format; nil keeps the plain-text responses. bodyLimit is the
// translator's request cap, which bounds how much of a chunked body is buffered.
func (s *SecurityAdapters) CreateTranslatorChainMiddleware(reject security.RejectFunc, bodyLimit int64) func(http.Handler) http.Handler {
	return s.chain(reject, func(a *security.Adapters) func(http.Handler) http.Handler {
		return a.CreateTranslatorChainMiddleware(reject, bodyLimit)
	})
