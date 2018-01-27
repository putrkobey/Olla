package handlers

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/app/handlers/dashboard"
	"github.com/putrkobey/Olla/internal/app/middleware"
)

// muxWithStatusRoute builds a mux wired the way registerRoutes wires it in
// production: the status handler is wrapped in middleware.GzipFunc before
// registration. The Application is the same one the wire-shape tests use, so
// the status handler has its real dependencies (repository, stats, registry,
