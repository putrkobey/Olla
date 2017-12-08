package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type dockerModelRunnerParser struct{}

// Parse transforms Docker Model Runner /engines/v1/models response into unified ModelInfo structures.
// DMR uses the standard OpenAI-compatible list format, so the structure mirrors llamaCppParser.
func (p *dockerModelRunnerParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response DockerModelRunnerResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Docker Model Runner response: %w", err)
	}

	models := make([]*domain.ModelInfo, 0, len(response.Data))
	now := time.Now()

	for _, model := range response.Data {
		if model.ID == "" {
			continue
		}
