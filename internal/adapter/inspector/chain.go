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

func NewChain(logger logger.StyledLogger) *Chain {
	return &Chain{
		inspectors: make([]ports.RequestInspector, 0, 4),
		logger:     logger,
	}
}

func (c *Chain) AddInspector(inspector ports.RequestInspector) {
	c.inspectors = append(c.inspectors, inspector)
}

func (c *Chain) Inspect(ctx context.Context, r *http.Request, targetPath string) (*domain.RequestProfile, error) {
	profile := domain.NewRequestProfile(targetPath)

	for _, inspector := range c.inspectors {
