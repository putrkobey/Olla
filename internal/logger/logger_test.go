package logger

import "testing"

func TestIsValidLevel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		level string
		want  bool
	}{
		{"debug", true},
