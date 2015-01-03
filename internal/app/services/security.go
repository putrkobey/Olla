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
