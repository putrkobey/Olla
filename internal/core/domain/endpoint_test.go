package domain_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestEndpoint_AuthHeaderValue_NotSerialised ensures that AuthHeaderValue never
// appears in JSON output. The field carries live credentials; exposing it through
// status endpoints or logs would be a security issue.
func TestEndpoint_AuthHeaderValue_NotSerialised(t *testing.T) {
	t.Parallel()

	ep := &domain.Endpoint{
