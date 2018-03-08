package config

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAuthConfig_RoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("bearer", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:  "bearer",
			Token: "${TOKEN}",
		}
		roundTrip(t, original, &AuthConfig{})
	})

	t.Run("bearer with file", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:      "bearer",
			TokenFile: "/run/secrets/token",
		}
		roundTrip(t, original, &AuthConfig{})
	})

	t.Run("api_key with custom header", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:   "api_key",
			Key:    "${API_KEY}",
			Header: "X-Custom-Key",
		}
		roundTrip(t, original, &AuthConfig{})
	})

	t.Run("api_key with file", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:    "api_key",
			KeyFile: "/run/secrets/key",
		}
		roundTrip(t, original, &AuthConfig{})
	})

	t.Run("basic", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:     "basic",
			Username: "user",
			Password: "pass",
		}
		roundTrip(t, original, &AuthConfig{})
	})

	t.Run("basic with files", func(t *testing.T) {
		t.Parallel()
		original := &AuthConfig{
			Type:         "basic",
			UsernameFile: "/run/secrets/user",
			PasswordFile: "/run/secrets/pass",
		}
		roundTrip(t, original, &AuthConfig{})
	})
}

func TestEndpointConfig_AuthAndHeaders_RoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("full endpoint with auth and headers", func(t *testing.T) {
		t.Parallel()
		original := &EndpointConfig{
			Name: "secure-llama",
			URL:  "http://llamabox.local:8080",
			Type: "llamacpp",
			Auth: &AuthConfig{
				Type:   "bearer",
				Token:  "${TOKEN}",
				Header: "X-Api-Key",
			},
			Headers: map[string]string{
				"X-Custom":  "value",
				"X-Another": "other",
			},
		}
		roundTrip(t, original, &EndpointConfig{})
	})

	t.Run("endpoint without auth or headers is backwards compatible", func(t *testing.T) {
		t.Parallel()
