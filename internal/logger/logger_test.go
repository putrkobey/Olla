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
