package app

import (
	"context"
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/app/services"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/logger"
)

// CreateAndStartServiceManager initialises the service orchestration layer, establishing
// the dependency graph and bootstrapping all services in the correct order. This ensures
// health checks and discovery run before the proxy accepts traffic, maintaining the
// original startup behaviour where endpoints are validated immediately.
func CreateAndStartServiceManager(ctx context.Context, cfg *config.Config, logger logger.StyledLogger) (*services.ServiceManager, error) {
	startTime := time.Now()
	defer func() {
		logger.Debug("Service manager startup completed", "duration", time.Since(startTime))
	}()

	manager := services.NewServiceManager(logger)

	if err := registerServices(manager, cfg, logger); err != nil {
		return nil, fmt.Errorf("failed to register services: %w", err)
	}

	// Services are started in dependency order, with stats initialised first,
	// followed by security and discovery (which runs initial health checks),
	// then proxy and finally HTTP. This preserves the critical startup sequence.
	if err := manager.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start services: %w", err)
	}

	return manager, nil
}

// registerServices establishes the service dependency graph using a two-phase approach:
// registration followed by dependency injection. This pattern allows circular dependency
// resolution and ensures services can reference each other without initialisation races.
func registerServices(manager *services.ServiceManager, cfg *config.Config, logger logger.StyledLogger) error {
	statsService := services.NewStatsService(logger)
	if err := manager.Register(statsService); err != nil {
		return fmt.Errorf("failed to register stats service: %w", err)
	}

	// Security service requires stats collector, but we defer resolution to avoid
	// accessing uninitialised components. The nil parameter is intentional.
	securityService := services.NewSecurityService(
		&cfg.Server,
		nil,
		logger,
	)
	if err := manager.Register(securityService); err != nil {
		return fmt.Errorf("failed to register security service: %w", err)
	}

	discoveryService := services.NewDiscoveryService(
		&cfg.Discovery,
		&cfg.ModelRegistry,
		nil,
		logger,
	)
	if err := manager.Register(discoveryService); err != nil {
		return fmt.Errorf("failed to register discovery service: %w", err)
	}
