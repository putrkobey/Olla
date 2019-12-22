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
