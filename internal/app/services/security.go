package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/putrkobey/Olla/internal/adapter/security"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// SecurityService provides request validation, rate limiting, and security policy
// enforcement. It creates a chain of inspectors that validate incoming requests
// before they reach the proxy layer.
type SecurityService struct {
	serverConfig   *config.ServerConfig
	statsCollector ports.StatsCollector
	statsService   *StatsService
	logger         logger.StyledLogger
	services       *security.Services
	adapters       *security.Adapters
}

// NewSecurityService creates a new security service
func NewSecurityService(serverConfig *config.ServerConfig, statsCollector ports.StatsCollector, logger logger.StyledLogger) *SecurityService {
	return &SecurityService{
		serverConfig:   serverConfig,
		statsCollector: statsCollector,
		logger:         logger,
	}
}

// Name returns the service name
func (s *SecurityService) Name() string {
	return "security"
}

// Start initialises security components
func (s *SecurityService) Start(ctx context.Context) error {
	s.logger.Info("Initialising security service")

	// Factory requires full config structure for historical reasons
	tempConfig := &config.Config{
		Server: *s.serverConfig,
