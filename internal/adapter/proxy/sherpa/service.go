package sherpa

//                                       Sherpa Proxy Implementation
//
// The Sherpa proxy implementation is a clean and pragmatic reverse proxy designed for handling AI inference workloads
// such as LLM and embedding requests. It prioritises readability, simplicity and reliability while providing essential
// support for streaming, timeout handling and observability. It was the foundation of the Sherpa AI Tooling which was
// originally based on Scout.
//
// Key design features:
// - **Connection reuse**: Uses a single shared `http.Transport` with reasonable TCP settings
//   (e.g. keep-alive, SetNoDelay) to reduce connection churn.
// - **Streaming support**: Optimised for long-lived HTTP responses via buffered reads and flushing,
//   with graceful handling of timeouts and client disconnects.
// - **Basic buffer pooling**: Reuses read buffers via `sync.Pool` to reduce heap churn during streaming.
// - **Request metadata tracking**: Records request lifecycle timings including header processing,
//   backend latency, and streaming duration.
// - **Stat collection**: Uses atomic counters and a pluggable `StatsCollector` to track success/failure rates and latency.
//
// Sherpa is suitable for:
// - Moderate-throughput inference services with stable upstreams
// - Environments prioritising maintainability and clarity over extreme performance (that's for Olla Proxy)
//
// Sherpa is *not* intended for:
// - High-throughput or low-latency scenarios where custom transports or advanced connection pooling are required
// - Complex routing or load balancing needs beyond basic endpoint selection
// - Environments where maximum performance is critical (use Olla Proxy for that)
// - Scenarios requiring advanced features like circuit breaking, rate limiting, etc.
// - Environments where the proxy itself must be highly resilient to failures (use Olla Proxy for that)

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/proxy/config"
	"github.com/putrkobey/Olla/internal/adapter/proxy/core"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/pkg/pool"
)

const (
	// Sherpa-specific constants that aren't in the base proxy package
	DefaultSetNoDelay         = true
	DefaultDisableCompression = false

	DefaultMaxIdleConns        = 20
	DefaultMaxIdleConnsPerHost = 5

	DefaultIdleConnTimeout            = 90 * time.Second
	DefaultTLSHandshakeTimeout        = 10 * time.Second
	ClientDisconnectionBytesThreshold = 1024
	ClientDisconnectionTimeThreshold  = 5 * time.Second
)

// DefaultResponseHeaderTimeout re-exports the shared constant for Sherpa callers.
const DefaultResponseHeaderTimeout = config.DefaultResponseHeaderTimeout

// Service implements the Sherpa proxy - optimised for simplicity and maintainability
type Service struct {
	*core.BaseProxyComponents
