package security

/*
				Olla Security Adapter - Rate Limit Validator
	RateLimitValidator enforces global and per-IP rate limits using token buckets.
	It supports custom limits for health check endpoints and includes automatic
	cleanup of stale IP limiters. We're trying to keep it simple and efficient.

	It's thread-safe and designed for high-throughput environments.

	References:
	- https://pkg.go.dev/golang.org/x/time/rate
	- https://datatracker.ietf.org/doc/draft-ietf-httpapi-ratelimit-headers/
*/

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/putrkobey/Olla/internal/core/constants"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/util"
	"golang.org/x/time/rate"
)

type RateLimitValidator struct {
	metrics ports.SecurityMetricsService
	logger  logger.StyledLogger

	globalLimiter           *rate.Limiter
	cleanupTicker           *time.Ticker
	stopCleanup             chan struct{}
	rejectLog               *logger.Throttle
	ipLimiters              *xsync.Map[string, *ipLimiterInfo]
	trustedCIDRs            []*net.IPNet
	globalRequestsPerMinute int
	perIPRequestsPerMinute  int
	burstSize               int
	healthRequestsPerMinute int
	stopOnce                sync.Once
	trustProxyHeaders       bool
}

// ipCharge is the per-IP token a request took. It is a value (not a closure)
// so the admit path, which is every successful request, stays allocation-free.
type ipCharge struct {
	at          time.Time
	reservation *rate.Reservation
	info        *ipLimiterInfo
}

// refund hands the token back; a no-op on the zero value.
func (c ipCharge) refund() {
	if c.reservation == nil {
		return
	}
	c.reservation.CancelAt(c.at)
	c.info.mu.Lock()
	if c.info.tokensUsed > 0 {
		c.info.tokensUsed--
	}
	c.info.mu.Unlock()
}

type ipLimiterInfo struct {
	lastAccess   time.Time
	windowStart  time.Time
	limiter      *rate.Limiter
	tokensUsed   int
	requestLimit int
	mu           sync.RWMutex
	// flow serialises one client's per-IP reserve, global decision and refund.
	// x/time's CancelAt only restores a token exactly when no other reservation
	// on that limiter has landed since, so interleaved requests from one client
	// could lose a token on refund. Other clients are unaffected.
	flow sync.Mutex
}

func NewRateLimitValidator(limits config.ServerRateLimits, metrics ports.SecurityMetricsService, log logger.StyledLogger) *RateLimitValidator {
	rl := &RateLimitValidator{
		globalRequestsPerMinute: limits.GlobalRequestsPerMinute,
		perIPRequestsPerMinute:  limits.PerIPRequestsPerMinute,
		burstSize:               limits.BurstSize,
		healthRequestsPerMinute: limits.HealthRequestsPerMinute,
		trustProxyHeaders:       limits.TrustProxyHeaders,
		trustedCIDRs:            limits.TrustedProxyCIDRsParsed,
		metrics:                 metrics,
		logger:                  log,
		stopCleanup:             make(chan struct{}),
		ipLimiters:              xsync.NewMap[string, *ipLimiterInfo](),
	}

	// A client over its limit can be rejected at wire speed, so the WARN is
	// sampled per client IP. The violation counters still see every rejection.
	rl.rejectLog = logger.NewRejectThrottle(log, "Rate limit exceeded", "rate_limit", "client_ip", nil)

	if limits.GlobalRequestsPerMinute > 0 {
		globalRate := rate.Limit(float64(limits.GlobalRequestsPerMinute) / 60.0)
		rl.globalLimiter = rate.NewLimiter(globalRate, limits.BurstSize)
	}

	if limits.CleanupInterval > 0 {
		rl.cleanupTicker = time.NewTicker(limits.CleanupInterval)
		go rl.cleanupRoutine()
	}

	return rl
}

func (rl *RateLimitValidator) Name() string {
	return "rate_limit"
}

// Validate checks if the request should be allowed under current rate limits.
// The per-IP and global tiers are independent switches (0 disables only that
// tier). The per-IP tier is consulted first so a client already over its own
// limit cannot burn global tokens and starve everyone else. If the per-IP tier
// admits the request but the global tier rejects it, the per-IP token is handed
// back so a rejected request is not charged to either bucket.
// Thread safe.
func (rl *RateLimitValidator) Validate(_ context.Context, req ports.SecurityRequest) (ports.SecurityResult, error) {
	now := time.Now()

	var limit int
	if req.IsHealthCheck {
		limit = rl.healthRequestsPerMinute
