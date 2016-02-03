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

