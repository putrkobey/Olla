package domain_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestAuthHintRoundTrip verifies that the auth hint section in a profile YAML
// deserialises correctly and that the zero value is safe (omitempty means the
// hint is simply absent when not configured).
func TestAuthHintRoundTrip(t *testing.T) {
	t.Parallel()

	yamlInput := `
name: test-profile
version: "1.0"
characteristics:
  timeout: 5m
  streaming_support: true
  auth:
    required: false
    types:
      - bearer
      - api_key
    default_header: "X-Api-Key"
`

	var cfg domain.ProfileConfig
	if err := yaml.Unmarshal([]byte(yamlInput), &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal failed: %v", err)
	}

	hint := cfg.Characteristics.Auth
