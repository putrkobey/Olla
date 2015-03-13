package unifier

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// ExtendedUnifier extends the basic ModelUnifier interface with additional methods
type ExtendedUnifier interface {
	ports.ModelUnifier
