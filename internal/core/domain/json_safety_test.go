package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestEndpointModelsURLNotSerialised asserts that EndpointModels.EndpointURL
// is never emitted in JSON output. The field is used as an internal map key
// and must not appear in API responses; it may carry auth credentials or
// internal network addresses.
func TestEndpointModelsURLNotSerialised(t *testing.T) {
	t.Parallel()

	em := domain.EndpointModels{
		LastUpdated: time.Now(),
		EndpointURL: "http://user:pass@192.168.1.100:8000",
		Models:      []*domain.ModelInfo{{Name: "llama3"}},
	}

	data, err := json.Marshal(em)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	if strings.Contains(string(data), "192.168.1.100") {
		t.Errorf("EndpointModels JSON contains endpoint URL: %s", data)
