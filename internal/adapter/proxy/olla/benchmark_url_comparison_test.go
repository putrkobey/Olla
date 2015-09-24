package olla

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/util"
)

// buildTargetURL_Old is the original implementation (for comparison)
func buildTargetURL_Old(r *http.Request, endpoint *domain.Endpoint, proxyPrefix string) *url.URL {
	targetPath := util.StripPrefix(r.URL.Path, proxyPrefix)
	targetURL := endpoint.URL.ResolveReference(&url.URL{Path: targetPath})
	if r.URL.RawQuery != "" {
		targetURL.RawQuery = r.URL.RawQuery
	}
	return targetURL
}

// buildTargetURL_New is the optimized implementation with fast path
func buildTargetURL_New(r *http.Request, endpoint *domain.Endpoint, proxyPrefix string) *url.URL {
	targetPath := util.StripPrefix(r.URL.Path, proxyPrefix)

	// Fast path: if endpoint URL has no path or only "/", avoid ResolveReference allocation
	if endpoint.URL.Path == "" || endpoint.URL.Path == "/" {
		targetURL := *endpoint.URL // Shallow copy is safe
		targetURL.Path = targetPath
		if r.URL.RawQuery != "" {
			targetURL.RawQuery = r.URL.RawQuery
		}
		return &targetURL
	}

	// Slow path: complex URL resolution
	targetURL := endpoint.URL.ResolveReference(&url.URL{Path: targetPath})
	if r.URL.RawQuery != "" {
		targetURL.RawQuery = r.URL.RawQuery
	}
	return targetURL
}

// BenchmarkBuildTargetURL_Comparison compares old vs new implementation
func BenchmarkBuildTargetURL_Comparison(b *testing.B) {
	// Test with simple endpoint (the common case)
