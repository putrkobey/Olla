package util

import (
	"net"
	"net/http/httptest"
	"testing"
)

func TestGenerateRequestID(t *testing.T) {
	id1 := GenerateRequestID()
	id2 := GenerateRequestID()

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}

	if len(id1) == 0 {
		t.Error("Generated ID should not be empty")
	}

	// Should follow the pattern: name_action_suffix
	parts := len(id1)
	if parts < 10 {
		t.Errorf("Generated ID seems too short: %s", id1)
	}
}

func TestGetClientIP_NoProxyHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"

	ip := GetClientIP(req, false, nil)
	if ip != "192.168.1.100" {
		t.Errorf("Expected 192.168.1.100, got %s", ip)
	}
}

func TestGetClientIP_WithProxyHeaders_TrustedSource(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345" // Trusted proxy IP
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.1")

	trustedCIDRs, _ := ParseTrustedCIDRs([]string{"192.168.0.0/16"})

	ip := GetClientIP(req, true, trustedCIDRs)
	if ip != "203.0.113.1" {
		t.Errorf("Expected 203.0.113.1 from X-Forwarded-For, got %s", ip)
	}
}

func TestGetClientIP_WithProxyHeaders_UntrustedSource(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "203.0.113.1:12345" // Untrusted external IP
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	trustedCIDRs, _ := ParseTrustedCIDRs([]string{"192.168.0.0/16"})

	ip := GetClientIP(req, true, trustedCIDRs)
	if ip != "203.0.113.1" {
		t.Errorf("Expected 203.0.113.1 (ignoring untrusted proxy headers), got %s", ip)
	}
}

func TestGetClientIP_XRealIP_TrustedSource(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.50")

	trustedCIDRs, _ := ParseTrustedCIDRs([]string{"10.0.0.0/8"})

	ip := GetClientIP(req, true, trustedCIDRs)
	if ip != "203.0.113.50" {
		t.Errorf("Expected 203.0.113.50 from X-Real-IP, got %s", ip)
	}
}

func TestGetClientIP_ProxyHeadersDisabled(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	trustedCIDRs, _ := ParseTrustedCIDRs([]string{"192.168.0.0/16"})

	ip := GetClientIP(req, false, trustedCIDRs)
	if ip != "192.168.1.1" {
		t.Errorf("Expected 192.168.1.1 (proxy headers disabled), got %s", ip)
	}
}

func TestGetClientIP_EmptyTrustedCIDRs(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	ip := GetClientIP(req, true, nil)
	if ip != "192.168.1.1" {
		t.Errorf("Expected 192.168.1.1 (no trusted CIDRs), got %s", ip)
	}
}

func TestParseTrustedCIDRs_Valid(t *testing.T) {
	cidrs := []string{
		"192.168.0.0/16",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"127.0.0.1/32",
	}

	networks, err := ParseTrustedCIDRs(cidrs)
	if err != nil {
		t.Fatalf("ParseTrustedCIDRs failed: %v", err)
	}

	if len(networks) != 4 {
		t.Errorf("Expected 4 networks, got %d", len(networks))
	}

	// Test that 192.168.1.100 is in the first network
	testIP := net.ParseIP("192.168.1.100")
	if !networks[0].Contains(testIP) {
		t.Error("192.168.1.100 should be in 192.168.0.0/16")
	}
}

func TestParseTrustedCIDRs_Invalid(t *testing.T) {
	cidrs := []string{
		"192.168.0.0/16",
		"invalid-cidr",
		"10.0.0.0/8",
	}

	_, err := ParseTrustedCIDRs(cidrs)
	if err == nil {
		t.Error("Expected error for invalid CIDR")
	}
}

func TestParseTrustedCIDRs_Empty(t *testing.T) {
	networks, err := ParseTrustedCIDRs([]string{})
	if err != nil {
		t.Fatalf("ParseTrustedCIDRs failed with empty slice: %v", err)
	}
	if networks != nil {
		t.Error("Expected nil for empty CIDR list")
	}
}

func TestParseTrustedCIDRs_WithSpaces(t *testing.T) {
	cidrs := []string{
		" 192.168.0.0/16 ",
		"  10.0.0.0/8",
		"172.16.0.0/12  ",
		"", // Empty string should be skipped
