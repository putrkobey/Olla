package unifier

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type DiscoveryClient interface {
	DiscoverModels(ctx context.Context, endpoint *domain.Endpoint) ([]*domain.ModelInfo, error)
}

// LifecycleUnifier manages model lifecycle with automatic cleanup,
// health monitoring, and circuit breaker support
type LifecycleUnifier struct {
	unifier         ports.ModelUnifier
	discoveryClient DiscoveryClient
	logger          logger.StyledLogger

	cleanupCtx      context.Context
	endpointManager *EndpointManager

	cleanupCancel context.CancelFunc
	config        Config
	cleanupWg     sync.WaitGroup
	isRunning     atomic.Bool
}

func NewLifecycleUnifier(config Config, logger logger.StyledLogger) ports.ModelUnifier {
	if err := config.Validate(); err != nil {
		config = DefaultConfig()
	}

	return &LifecycleUnifier{
		unifier:         NewDefaultUnifier(),
		endpointManager: NewEndpointManager(config, logger),
		config:          config,
		logger:          logger,
	}
}

func (u *LifecycleUnifier) SetDiscoveryClient(client DiscoveryClient) {
	u.discoveryClient = client
}

func (u *LifecycleUnifier) Start(ctx context.Context) error {
	if !u.isRunning.CompareAndSwap(false, true) {
		return errors.New("unifier is already running")
	}

	u.cleanupCtx, u.cleanupCancel = context.WithCancel(ctx)

	if u.config.EnableBackgroundCleanup {
		u.cleanupWg.Add(1)
		go u.cleanupRoutine()
	}

	u.logger.Info("Lifecycle unifier started",
		"model_ttl", u.config.ModelTTL,
		"cleanup_interval", u.config.CleanupInterval,
