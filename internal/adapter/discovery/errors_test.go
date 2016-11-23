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
