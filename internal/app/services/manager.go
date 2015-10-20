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

func (sm *ServiceManager) Register(service ManagedService) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	name := service.Name()
	if _, exists := sm.services[name]; exists {
		return fmt.Errorf("service %s already registered", name)
	}

	sm.services[name] = service
	sm.registry.Register(name, service)
	sm.logger.Debug("Service registered", "name", name)
	return nil
}

// resolveDependencies implements Kahn's algorithm for topological sorting to determine
// the correct service startup order. Returns an error if circular dependencies are
// detected or if a service declares a dependency on a non-existent service.
func (sm *ServiceManager) resolveDependencies() ([]string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	dependencies := make(map[string][]string)
	inDegree := make(map[string]int)

	for name, service := range sm.services {
		dependencies[name] = service.Dependencies()
		inDegree[name] = 0
	}

	for _, deps := range dependencies {
		for _, dep := range deps {
			if _, exists := sm.services[dep]; !exists {
				return nil, fmt.Errorf("dependency %s not registered", dep)
			}
			inDegree[dep]++
		}
	}

	var order []string
	queue := make([]string, 0)

	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
