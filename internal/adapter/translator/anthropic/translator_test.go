package anthropic

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
)

// TestGetAPIPath tests the PathProvider interface implementation
func TestGetAPIPath(t *testing.T) {
	translator := mustNewTranslator(createTestLogger(), createTestConfig())

	path := translator.GetAPIPath()
	assert.Equal(t, "/olla/anthropic/v1/messages", path, "should return the correct Anthropic API path")
}

// TestWriteError tests the ErrorWriter interface implementation
func TestWriteError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		statusCode     int
		expectedType   string
		expectedStatus int
	}{
		{
			name:           "bad_request",
			err:            errors.New("invalid model specified"),
			statusCode:     http.StatusBadRequest,
			expectedType:   "invalid_request_error",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unauthorized",
			err:            errors.New("missing API key"),
			statusCode:     http.StatusUnauthorized,
			expectedType:   "authentication_error",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "forbidden",
			err:            errors.New("insufficient permissions"),
			statusCode:     http.StatusForbidden,
			expectedType:   "permission_error",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "not_found",
			err:            errors.New("model not found"),
			statusCode:     http.StatusNotFound,
			expectedType:   "not_found_error",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "rate_limit",
			err:            errors.New("rate limit exceeded"),
			statusCode:     http.StatusTooManyRequests,
			expectedType:   "rate_limit_error",
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			// 503 from a backend means the backend is unavailable, not that Anthropic's
			// own infra is overloaded. overloaded_error is an Anthropic-origin signal;
			// a generic gateway 503 maps to api_error per litellm/bifrost conventions.
			name:           "service_unavailable",
			err:            errors.New("service overloaded"),
			statusCode:     http.StatusServiceUnavailable,
			expectedType:   "api_error",
			expectedStatus: http.StatusServiceUnavailable,
