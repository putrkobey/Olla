package discovery

import (
	"errors"
	"testing"
	"time"
)

func TestGetUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name: "network error with connectex",
			err: NewDiscoveryError(
				"http://192.168.0.113:11434",
				"ollama",
				"http_request",
				0,
				1*time.Millisecond,
				&NetworkError{
					URL: "http://192.168.0.113:11434/api/tags",
					Err: errors.New("dial tcp 192.168.0.113:11434: connectex: A socket operation was attempted to an unreachable host."),
				},
			),
			expected: "endpoint unreachable",
		},
		{
			name: "http 404 error",
			err: NewDiscoveryError(
				"http://localhost:11434",
				"ollama",
				"http_status",
				404,
				2*time.Millisecond,
				errors.New("HTTP 404: Not Found"),
			),
			expected: "endpoint configuration issue (HTTP 404)",
		},
		{
			name: "http 500 error",
			err: NewDiscoveryError(
				"http://localhost:11434",
				"ollama",
				"http_status",
				500,
				2*time.Millisecond,
				errors.New("HTTP 500: Internal Server Error"),
			),
			expected: "endpoint server error (HTTP 500)",
		},
		{
			name: "timeout error",
			err: NewDiscoveryError(
				"http://localhost:11434",
				"ollama",
				"http_request",
				0,
				5*time.Second,
				errors.New("context deadline exceeded"),
			),
			expected: "connection timeout",
		},
