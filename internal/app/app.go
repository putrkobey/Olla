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
