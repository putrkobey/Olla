package util

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func isIPInTrustedCIDRs(ip net.IP, trustedCIDRs []*net.IPNet) bool {
	for _, cidr := range trustedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func ParseTrustedCIDRs(cidrStrings []string) ([]*net.IPNet, error) {
	if len(cidrStrings) == 0 {
		return nil, nil
	}

	var cidrs []*net.IPNet
	for _, cidrStr := range cidrStrings {
		cidrStr = strings.TrimSpace(cidrStr)
		if cidrStr == "" {
			continue
		}
