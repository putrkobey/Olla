package converter

import (
	"strings"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type LMDeployModelResponse = profile.LMDeployResponse
type LMDeployModelData = profile.LMDeployModel

// LMDeployConverter converts models to LMDeploy-compatible format.
// LMDeploy's /v1/models shape is OpenAI-compatible but without max_model_len
// and with owned_by defaulting to "lmdeploy".
type LMDeployConverter struct {
	*BaseConverter
}

// NewLMDeployConverter creates a new LMDeploy format converter.
