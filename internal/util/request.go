package util

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
)

func GenerateRequestID() string {
	actions := []string{
		"grazing", "trekking", "humming", "spitting", "prancing",
		"carrying", "leading", "following", "resting", "alerting",
		"browsing", "foraging", "wandering", "galloping", "ambling",
	}
	llamas := []string{
		"huacaya", "suri", "vicuna", "alpaca", "guanaco",
		"woolly", "silky", "fluffy", "curly", "shaggy",
		"noble", "gentle", "swift", "steady", "proud",
	}

	group := llamas[rand.Intn(len(llamas))]
	action := actions[rand.Intn(len(actions))]
	suffix := fmt.Sprintf("%04x", rand.Intn(65536))

	return fmt.Sprintf("%s-%s-%s", group, action, suffix)
}

func GetClientIP(r *http.Request, trustProxyHeaders bool, trustedCIDRs []*net.IPNet) string {
	if !trustProxyHeaders {
		if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return ip
		}
		return r.RemoteAddr
	}

	// With no trusted CIDRs configured, every source is implicitly untrusted and
	// getSourceIP would allocate a net.IP only to immediately discard it. Skip the
	// parse and fall through to RemoteAddr directly.
	if len(trustedCIDRs) == 0 {
		if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
			return ip
		}
		return r.RemoteAddr
