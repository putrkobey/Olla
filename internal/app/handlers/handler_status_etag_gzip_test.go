package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/app/middleware"
)

// TestStatusETag_GzipRequestReturnsBare304 is the critical B1xB2 interaction
// assertion: a gzip-negotiated request that hits the ETag path (its second
// sequential poll carries a matching If-None-Match) must return a BARE 304:
//
//   - status 304 Not Modified
//   - ETag header echoing the validator the client sent
//   - NO Content-Encoding header (a 304 has no body to compress)
//   - empty body
//
// This property is what keeps the dashboard's polling loop cheap: the poll
// round-trips a few hundred bytes for a 304 instead of the full gzipped
