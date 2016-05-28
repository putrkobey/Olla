package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadRetryYAML(t *testing.T, body string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "olla.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return Load(path)
}

func TestProxyRetry_Config(t *testing.T) {
	t.Parallel()

	defaults := RetryConfig{Enabled: true, MaxAttempts: 0, NonIdempotent: true}

	tests := []struct {
		name    string
		yaml    string
		want    RetryConfig
		wantErr string
	}{
		{name: "no proxy block keeps today's behaviour", yaml: "server:\n  port: 40114\n", want: defaults},
		{name: "empty retry block keeps today's behaviour", yaml: "proxy:\n  retry: {}\n", want: defaults},
		{name: "partial block keeps the other defaults", yaml: "proxy:\n  retry:\n    max_attempts: 2\n", want: RetryConfig{Enabled: true, MaxAttempts: 2, NonIdempotent: true}},
		{name: "enabled false parses", yaml: "proxy:\n  retry:\n    enabled: false\n", want: RetryConfig{Enabled: false, NonIdempotent: true}},
		{name: "non_idempotent false parses", yaml: "proxy:\n  retry:\n    non_idempotent: false\n", want: RetryConfig{Enabled: true, NonIdempotent: false}},
		{name: "negative max_attempts is rejected", yaml: "proxy:\n  retry:\n    max_attempts: -1\n", wantErr: "proxy.retry.max_attempts must be >= 0"},
		{name: "deprecated keys do not touch the policy", yaml: "proxy:\n  max_retries: 3\n  retry_backoff: 1s\n", want: defaults},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

