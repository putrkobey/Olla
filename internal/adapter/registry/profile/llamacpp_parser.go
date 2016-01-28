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
	if len(data) == 0 {
		// looks like there's no models available
		return make([]*domain.ModelInfo, 0), nil
	}

	// Decode only fields discovery uses. Upstream metadata types can change
	// independently, and the duplicate Ollama-compatible models array is unused.
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
			Created int64  `json:"created"`
		} `json:"data"`
	}
	const LlamaCpp = "llamacpp"
