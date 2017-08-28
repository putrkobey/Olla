package routing

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// TestStrictStrategy_GetRoutableEndpoints covers the three outcomes strict routing
// can produce (#191): a healthy match routes, a match confined to unhealthy endpoints
// rejects with 503, and no match anywhere rejects with 404. Handlers rely on these
// exact status codes to fail fast instead of falling through to the proxy engine.
func TestStrictStrategy_GetRoutableEndpoints(t *testing.T) {
	ctx := context.Background()
	testLogger := createTestLogger()
