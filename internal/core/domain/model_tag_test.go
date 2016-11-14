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
		{"all-minilm:latest", "", false},
		{"qwen2.5:0.5b", "", false},
		{"library/foo", "library/foo:latest", true},
		{"registry.example.com:5000/foo", "registry.example.com:5000/foo:latest", true},
		{"registry.example.com:5000/foo:7b", "", false},
		{"", "", false},
		{"foo:", "", false},
	}
