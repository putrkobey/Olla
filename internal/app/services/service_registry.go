package services

import (
	"errors"
	"fmt"
)

// ServiceRegistry facilitates runtime service discovery and dependency injection
// after the registration phase completes.
type ServiceRegistry struct {
	services map[string]ManagedService
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]ManagedService),
	}
}

func (r *ServiceRegistry) Register(name string, service ManagedService) {
	r.services[name] = service
}

func (r *ServiceRegistry) Get(name string) (ManagedService, error) {
	service, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s not found", name)
	}
	return service, nil
}

func (r *ServiceRegistry) GetStats() (*StatsService, error) {
	service, err := r.Get("stats")
	if err != nil {
		return nil, err
	}
	stats, ok := service.(*StatsService)
	if !ok {
		return nil, errors.New("service stats is not a StatsService")
	}
	return stats, nil
}

func (r *ServiceRegistry) GetSecurity() (*SecurityService, error) {
