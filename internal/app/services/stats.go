package services

import (
	"context"
	"errors"

	"github.com/putrkobey/Olla/internal/adapter/stats"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// StatsService provides centralised metrics collection for all system components.
// It initialises early in the startup sequence as most other services depend on it
// for instrumentation and observability.
type StatsService struct {
