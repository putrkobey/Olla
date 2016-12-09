package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOllamaDefaultTagName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
		ok   bool
	}{
		{"all-minilm", "all-minilm:latest", true},
