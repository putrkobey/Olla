package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestDashboardConfig_Validate_DisabledShortCircuits(t *testing.T) {
	t.Parallel()

	c := DashboardConfig{Enabled: false}
	if err := c.Validate(); err != nil {
		t.Fatalf("disabled dashboard must validate clean, got %v", err)
	}
	if c.ParsedCIDRs() != nil {
		t.Fatalf("disabled dashboard must not populate parsed CIDRs, got %v", c.ParsedCIDRs())
	}
}

func TestDashboardConfig_Validate_EnabledRequiresCIDRs(t *testing.T) {
	t.Parallel()

	c := DashboardConfig{
		Enabled:      true,
		AccessPolicy: AccessPolicyConfig{AllowedHosts: []string{"localhost"}},
	}
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "access_policy.allowed_cidrs must not be empty") {
		t.Fatalf("expected error naming allowed_cidrs, got %v", err)
	}
	if !strings.Contains(err.Error(), "dashboard config invalid") {
		t.Fatalf("expected root-level wrap marker, got %v", err)
	}
}

// TestDashboardConfig_Validate_EmptyHostsAllowed confirms there is no
// "hosts must be non-empty" rule: IP-literal Hosts are always accepted,
// so an empty allowed_hosts is a legitimate config.
func TestDashboardConfig_Validate_EmptyHostsAllowed(t *testing.T) {
	t.Parallel()

	c := DashboardConfig{
		Enabled: true,
		AccessPolicy: AccessPolicyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8"},
			AllowedHosts: nil,
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("IP-literal hosts are auto-accepted, so empty allowed_hosts must validate clean, got %v", err)
	}
}

func TestDashboardConfig_Validate_BadCIDRFailsStartup(t *testing.T) {
	t.Parallel()

	c := DashboardConfig{
		Enabled: true,
		AccessPolicy: AccessPolicyConfig{
			AllowedCIDRs: []string{"127.0.0.0/8", "not-a-cidr"},
			AllowedHosts: []string{"localhost"},
		},
	}
