package security

import (
	"net/http"

	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type Services struct {
	Chain   *ports.SecurityChain
	Metrics ports.SecurityMetricsService
}

type Adapters struct {
	RateLimit      *RateLimitValidator
	SizeValidation *SizeValidator
	Metrics        *MetricsAdapter
	Chain          *ports.SecurityChain
}

// NewSecurityServices Creates and wires security validators so they're easy to chain and c onsume
func NewSecurityServices(cfg *config.Config, statsCollector ports.StatsCollector, logger logger.StyledLogger) (*Services, *Adapters) {
	metricsAdapter := NewSecurityMetricsAdapter(statsCollector, logger)
	rateLimitValidator := NewRateLimitValidator(cfg.Server.RateLimits, metricsAdapter, logger)
	sizeValidator := NewSizeValidator(cfg.Server.RequestLimits, metricsAdapter, logger)

	chain := ports.NewSecurityChain(
		rateLimitValidator, /* We start with rate limiting */
		sizeValidator,      /* if we pass that, we can check size */
	)

	services := &Services{
		Chain:   chain,
		Metrics: metricsAdapter,
	}

	adapters := &Adapters{
		RateLimit:      rateLimitValidator,
		SizeValidation: sizeValidator,
		Metrics:        metricsAdapter,
		Chain:          chain,
	}

	return services, adapters
}

func (sa *Adapters) Stop() {
	if sa.RateLimit != nil {
		sa.RateLimit.Stop()
	}
	if sa.SizeValidation != nil {
		sa.SizeValidation.Flush()
	}
}

// CreateChainMiddleware builds the rate-limit -> size-validation chain once,
// at wiring time. Building it fresh on every request (as this used to do)
