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

