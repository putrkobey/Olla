package router

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/pterm/pterm"
	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/logger"
)

type RouteInfo struct {
	Handler     http.HandlerFunc
	Reject      security.RejectFunc
	Description string
	Method      string
	Order       int
	IsProxy     bool
	// IsTranslator marks an inference route served by an API translator. It gets
	// the same rate and size limits as proxy routes, with rejections written by Reject.
	IsTranslator bool
	// BodyLimit is the translator's own request cap, used to bound how much of a chunked
	// body the size middleware buffers. Zero means no cap beyond the server-wide one.
	BodyLimit int64
}

type RouteRegistry struct {
	routes   map[string]RouteInfo
	logger   logger.StyledLogger
	orderSeq int
}

func NewRouteRegistry(logger logger.StyledLogger) *RouteRegistry {
	return &RouteRegistry{
		routes:   make(map[string]RouteInfo),
		logger:   logger,
		orderSeq: 0,
	}
}

func (r *RouteRegistry) Register(route string, handler http.HandlerFunc, description string) {
	r.RegisterWithMethod(route, handler, description, "GET")
}

func (r *RouteRegistry) RegisterWithMethod(route string, handler http.HandlerFunc, description, method string) {
	r.registerWithMethod(route, handler, description, method, false)
}

func (r *RouteRegistry) RegisterProxyRoute(route string, handler http.HandlerFunc, description string, method string) {
	wrappedHandler := func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), constants.ContextRoutePrefixKey, route)
		handler(w, req.WithContext(ctx))
	}
	r.registerWithMethod(route, wrappedHandler, description, method, true)
}

// RegisterTranslatorRoute registers a route served by an API translator (for example
// /olla/anthropic/v1/messages). These are inference entry points, so they sit behind
// the same rate and size limits as proxy routes. reject lets the translator answer a
// rejection in its own error schema; nil falls back to plain text. bodyLimit is the
// translator's request cap (0 for none).
func (r *RouteRegistry) RegisterTranslatorRoute(route string, handler http.HandlerFunc, description, method string, reject security.RejectFunc, bodyLimit int64) {
	r.registerWithMethod(route, handler, description, method, false)
	info := r.routes[route]
	info.IsTranslator = true
	info.Reject = reject
	info.BodyLimit = bodyLimit
	r.routes[route] = info
}

func (r *RouteRegistry) registerWithMethod(route string, handler http.HandlerFunc, description, method string, isProxy bool) {
	r.routes[route] = RouteInfo{
		Handler:     handler,
		Description: description,
		Method:      method,
		Order:       r.orderSeq,
