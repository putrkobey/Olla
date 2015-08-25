package services

import (
	"testing"

	proxyconfig "github.com/putrkobey/Olla/internal/adapter/proxy/config"
	appconfig "github.com/putrkobey/Olla/internal/config"
)

// The YAML-facing RetryConfig is positive (enabled, non_idempotent) while the
// engine policy is inverted so its zero value is today's behaviour; this pins
// the mapping between them.
func TestCreateProxyConfiguration_RetryPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   appconfig.RetryConfig
		want proxyconfig.RetryPolicy
