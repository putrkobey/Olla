package inspector

import (
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

type Factory struct {
	profileFactory *profile.Factory
	logger         logger.StyledLogger
}

func NewFactory(profileFactory *profile.Factory, logger logger.StyledLogger) *Factory {
