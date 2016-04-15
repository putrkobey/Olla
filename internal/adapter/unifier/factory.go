package unifier

import (
	"fmt"
	"sync"

	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

const (
	DefaultUnifierType   = "default"
	LifecycleUnifierType = "lifecycle"
)

// Factory creates model unifier instances
type Factory struct {
	creators map[string]func(log logger.StyledLogger) ports.ModelUnifier
	logger   logger.StyledLogger
	mu       sync.RWMutex
}

// NewFactory creates a new unifier factory with default registrations
func NewFactory(log logger.StyledLogger) *Factory {
	factory := &Factory{
		creators: make(map[string]func(log logger.StyledLogger) ports.ModelUnifier),
		logger:   log,
	}

	// Force the model unification config to load now, at startup, rather than
	// lazily on first inference request - a broken models.yaml should show up
	// in the boot logs, not silently degrade unification quality later.
	LogConfigStatus(log)

	// Register default unifier
	factory.Register(DefaultUnifierType, func(l logger.StyledLogger) ports.ModelUnifier {
		return NewDefaultUnifier()
	})

	// Register lifecycle unifier
	factory.Register(LifecycleUnifierType, func(l logger.StyledLogger) ports.ModelUnifier {
		return NewLifecycleUnifier(DefaultConfig(), l)
	})

	return factory
}

// Register adds a new unifier type to the factory
func (f *Factory) Register(name string, creator func(log logger.StyledLogger) ports.ModelUnifier) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creators[name] = creator
}

// Create instantiates a unifier of the specified type
func (f *Factory) Create(name string) (ports.ModelUnifier, error) {
	f.mu.RLock()
	creator, exists := f.creators[name]
	f.mu.RUnlock()
