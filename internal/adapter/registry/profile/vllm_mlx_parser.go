package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type vllmMLXParser struct{}

// Parse transforms vLLM-MLX /v1/models response into unified ModelInfo structures.
// vLLM-MLX exclusively serves MLX models, so the format hint is always set to RecipeMLX.
func (p *vllmMLXParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response VLLMMLXResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse vLLM-MLX response: %w", err)
	}

