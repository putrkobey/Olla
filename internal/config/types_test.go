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
