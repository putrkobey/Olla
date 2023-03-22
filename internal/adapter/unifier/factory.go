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
