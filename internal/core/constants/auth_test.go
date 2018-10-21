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
