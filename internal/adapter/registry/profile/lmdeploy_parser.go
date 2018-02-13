package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type lmdeployParser struct{}

func (p *lmdeployParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response LMDeployResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse LMDeploy response: %w", err)
	}

