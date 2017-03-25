package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type sglangParser struct{}

func (p *sglangParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}
