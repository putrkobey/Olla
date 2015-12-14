package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// benchNextHandler is the terminal handler both benchmarks wrap - representative
// of a typical status/health handler, not the proxy hot path itself.
