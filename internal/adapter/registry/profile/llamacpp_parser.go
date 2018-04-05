package profile

import (
	"fmt"
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

type llamaCppParser struct{}

// Parse transforms llama.cpp /v1/models response into unified ModelInfo structures
// llama.cpp serves GGUF models exclusively and uses OpenAI-compatible format
func (p *llamaCppParser) Parse(data []byte) ([]*domain.ModelInfo, error) {
