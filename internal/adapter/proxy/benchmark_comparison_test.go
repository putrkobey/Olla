package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/olla"
	"github.com/putrkobey/Olla/internal/adapter/proxy/sherpa"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// BenchmarkProxyComparison compares the refactored implementations
func BenchmarkProxyComparison(b *testing.B) {
	// Set up test server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	}))
	defer upstream.Close()

	endpoint := createTestEndpoint("test", upstream.URL, domain.StatusHealthy)
	endpoints := []*domain.Endpoint{endpoint}

	// Create refactored proxies
	refactoredSherpa, err := createRefactoredSherpaProxy(endpoints)
	if err != nil {
		b.Fatal(err)
	}

	refactoredOlla, err := createRefactoredOllaProxy(endpoints)
	if err != nil {
		b.Fatal(err)
	}
