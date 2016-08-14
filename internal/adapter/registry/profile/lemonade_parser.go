package profile

import (
	"fmt"
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type lemonadeParser struct{}

func (p *lemonadeParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
	if len(data) == 0 {
		return make([]*domain.ModelInfo, 0), nil
	}

	var response LemonadeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse Lemonade response: %w", err)
	}

	models := make([]*domain.ModelInfo, 0, len(response.Data))
	now := time.Now()

	for _, model := range response.Data {
		if model.ID == "" {
			continue
		}

		modelInfo := &domain.ModelInfo{
			Name:     model.ID,
			LastSeen: now,
		}

		// Lemonade provides extended metadata beyond standard OpenAI
		details := &domain.ModelDetails{}
		hasDetails := false

		// Set creation time if available
		if model.Created > 0 {
			createdTime := time.Unix(model.Created, 0)
			details.ModifiedAt = &createdTime
			hasDetails = true
		}

		// Store checkpoint (HuggingFace model path)
		// Critical for Lemonade's model identification and loading
		if model.Checkpoint != "" {
			details.Checkpoint = &model.Checkpoint
			hasDetails = true

