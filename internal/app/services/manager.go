package services

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/putrkobey/Olla/internal/logger"
)

// ManagedService defines the contract for services participating in the orchestration
// lifecycle. Services must be idempotent for Start/Stop operations and explicitly
// declare their dependencies to enable proper initialisation ordering.
type ManagedService interface {
	// Name returns the unique name of the service
	Name() string

	// Start initialises and starts the service
	Start(ctx context.Context) error

	// Stop gracefully shuts down the service
	Stop(ctx context.Context) error

	// Dependencies returns the names of services this service depends on
	Dependencies() []string
}

// ServiceManager orchestrates service lifecycle using topological sorting to resolve
// dependencies. It ensures services start in the correct order and shutdown gracefully
// in reverse order, handling partial startup failures with appropriate cleanup.
type ServiceManager struct {
	services   map[string]ManagedService
	registry   *ServiceRegistry
	logger     logger.StyledLogger
	startOrder []string // dependency-resolved start order
	mu         sync.RWMutex
}

// NewServiceManager creates a new service manager
func NewServiceManager(logger logger.StyledLogger) *ServiceManager {
	return &ServiceManager{
		services: make(map[string]ManagedService),
		registry: NewServiceRegistry(),
		logger:   logger,
	}
}
