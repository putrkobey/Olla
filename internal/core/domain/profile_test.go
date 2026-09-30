package domain_test

import (
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestLlamaCppProfileConstant(t *testing.T) {
	expected := "llamacpp"
	if domain.ProfileLlamaCpp != expected {
		t.Errorf("ProfileLlamaCpp: expected %q, got %q", expected, domain.ProfileLlamaCpp)
	}
}
