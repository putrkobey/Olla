package constants_test

import (
	"testing"

	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestOMLXProviderConstants(t *testing.T) {
	t.Parallel()

	t.Run("provider type constant", func(t *testing.T) {
		t.Parallel()
		expected := "omlx"
		if constants.ProviderTypeOMLX != expected {
			t.Errorf("ProviderTypeOMLX: expected %q, got %q", expected, constants.ProviderTypeOMLX)
		}
	})

	t.Run("display name constant", func(t *testing.T) {
		t.Parallel()
		expected := "oMLX"
		if constants.ProviderDisplayOMLX != expected {
			t.Errorf("ProviderDisplayOMLX: expected %q, got %q", expected, constants.ProviderDisplayOMLX)
		}
	})
