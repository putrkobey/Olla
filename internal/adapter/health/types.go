package health

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
)

const (
	DefaultHealthCheckerTimeout = 5 * time.Second
	SlowResponseThreshold       = 10 * time.Second

