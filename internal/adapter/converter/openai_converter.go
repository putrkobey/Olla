package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// OpenAIModelResponse represents the OpenAI-compatible format response
type OpenAIModelResponse struct {
	Object string            `json:"object"`
	Data   []OpenAIModelData `json:"data"`
}

// OpenAIModelData represents a single model in OpenAI format
type OpenAIModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
	Created int64  `json:"created"`
}

