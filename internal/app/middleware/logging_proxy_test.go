package middleware

import "testing"

func TestIsProxyRequest(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		// Proxy requests
		{
			name:     "olla proxy ollama path",
			path:     "/olla/ollama/api/chat",
			expected: true,
		},
		{
			name:     "olla proxy lmstudio path",
			path:     "/olla/lmstudio/v1/models",
			expected: true,
		},
		{
			name:     "olla proxy generic path",
			path:     "/olla/proxy/some/endpoint",
			expected: true,
		},
