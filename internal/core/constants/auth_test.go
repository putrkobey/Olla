package constants_test

import (
	"testing"

	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestIsValidAuthType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"bearer", constants.AuthTypeBearer, true},
		{"api_key", constants.AuthTypeAPIKey, true},
		{"basic", constants.AuthTypeBasic, true},
		{"empty", "", false},
		{"unknown", "oauth2", false},
		{"case sensitive", "Bearer", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := constants.IsValidAuthType(tt.input)
			if got != tt.want {
				t.Errorf("IsValidAuthType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAuthConstants(t *testing.T) {
	t.Parallel()

	t.Run("auth type values", func(t *testing.T) {
		t.Parallel()
		if constants.AuthTypeBearer != "bearer" {
			t.Errorf("AuthTypeBearer: expected %q, got %q", "bearer", constants.AuthTypeBearer)
		}
		if constants.AuthTypeAPIKey != "api_key" {
			t.Errorf("AuthTypeAPIKey: expected %q, got %q", "api_key", constants.AuthTypeAPIKey)
		}
		if constants.AuthTypeBasic != "basic" {
			t.Errorf("AuthTypeBasic: expected %q, got %q", "basic", constants.AuthTypeBasic)
		}
	})

	t.Run("header names", func(t *testing.T) {
		t.Parallel()
