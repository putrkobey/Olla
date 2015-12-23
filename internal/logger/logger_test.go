package logger

import "testing"

func TestIsValidLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level string
		want  bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"warning", true},
		{"error", true},
		{"DEBUG", true},
		{"Info", true},
		{"", false},
		{"trace", false},
		{"verbose", false},
		{"fatal", false}, // recognised by the level constants but not a slog handler level
	}

