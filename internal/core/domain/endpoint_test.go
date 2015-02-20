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
		Name:            "secure-ep",
		AuthHeaderName:  "Authorization",
		AuthHeaderValue: "Bearer super-secret-token",
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	if strings.Contains(string(data), "super-secret-token") {
		t.Errorf("AuthHeaderValue leaked into JSON output: %s", string(data))
	}
	if strings.Contains(string(data), "AuthHeaderValue") {
		t.Errorf("AuthHeaderValue field name present in JSON output: %s", string(data))
	}
}
