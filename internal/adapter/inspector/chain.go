package inspector

import (
	"context"
	"net/http"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type Chain struct {
	logger     logger.StyledLogger
	inspectors []ports.RequestInspector
}
