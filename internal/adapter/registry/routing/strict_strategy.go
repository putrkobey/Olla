package routing

import (
	"context"
	"fmt"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// StrictStrategy only routes to endpoints that have the model
type StrictStrategy struct {
	logger logger.StyledLogger
}

// NewStrictStrategy creates a new strict routing strategy
func NewStrictStrategy(logger logger.StyledLogger) *StrictStrategy {
	return &StrictStrategy{
		logger: logger,
	}
}

// Name returns the strategy name
