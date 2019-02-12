package olla

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/common"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// BenchmarkBuildTargetURL benchmarks URL building with fast path vs slow path
func BenchmarkBuildTargetURL(b *testing.B) {
	// Create a test service
	config := &Configuration{}
	config.ResponseTimeout = 30 * time.Second
	config.ReadTimeout = 10 * time.Second
	config.StreamBufferSize = 8192
	config.MaxIdleConns = 200
	config.IdleConnTimeout = 90 * time.Second
	config.MaxConnsPerHost = 50

