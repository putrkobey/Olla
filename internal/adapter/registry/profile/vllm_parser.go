package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type vllmParser struct{}

func (p *vllmParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response VLLMResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse vLLM response: %w", err)
	}

	models := make([]*domain.ModelInfo, 0, len(response.Data))
	now := time.Now()

