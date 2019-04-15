package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type LlamaCppResponse = profile.LlamaCppResponse
type LlamaCppModel = profile.LlamaCppModel

