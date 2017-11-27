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
	return &Factory{
		profileFactory: profileFactory,
		logger:         logger,
	}
}

func (f *Factory) CreateDefaultChain() ports.InspectorChain {
	chain := NewChain(f.logger)

	pathInspector := NewPathInspector(f.profileFactory, f.logger)
	chain.AddInspector(pathInspector)

	f.logger.Debug("Created inspector chain", "inspectors", []string{pathInspector.Name()})

