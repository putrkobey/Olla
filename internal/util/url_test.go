package util

import "testing"

func TestResolveURLPath(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		pathOrURL string
		expected  string
	}{
		{
			name:      "base with trailing slash, path with leading slash",
			baseURL:   "http://localhost:12434/engines/llama.cpp/",
			pathOrURL: "/v1/models",
			expected:  "http://localhost:12434/engines/llama.cpp/v1/models",
		},
		{
			name:      "base without trailing slash, path with leading slash",
			baseURL:   "http://localhost:11434",
			pathOrURL: "/api/tags",
			expected:  "http://localhost:11434/api/tags",
		},
		{
			name:      "base with trailing slash, path without leading slash",
			baseURL:   "http://localhost:12434/api/",
			pathOrURL: "v1/models",
			expected:  "http://localhost:12434/api/v1/models",
		},
		{
			name:      "base without trailing slash, path without leading slash",
			baseURL:   "http://localhost:11434",
			pathOrURL: "api/tags",
			expected:  "http://localhost:11434/api/tags",
		},
		{
			name:      "empty base",
			baseURL:   "",
			pathOrURL: "/v1/models",
			expected:  "/v1/models",
		},
		{
			name:      "empty path",
			baseURL:   "http://localhost:11434",
			pathOrURL: "",
			expected:  "http://localhost:11434",
		},
		{
			name:      "absolute URL with http scheme",
			baseURL:   "http://localhost:12434/api/",
			pathOrURL: "http://other-host:9000/v1/models",
			expected:  "http://other-host:9000/v1/models",
		},
		{
			name:      "absolute URL with https scheme",
			baseURL:   "http://localhost:12434/api/",
			pathOrURL: "https://api.example.com/models",
			expected:  "https://api.example.com/models",
		},
		{
			name:      "absolute URL overrides base completely",
			baseURL:   "http://localhost:11434",
			pathOrURL: "https://api.openai.com/v1/models",
			expected:  "https://api.openai.com/v1/models",
		},
