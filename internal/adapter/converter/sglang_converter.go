package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// Type aliases for backward compatibility with tests
type SGLangModelResponse = profile.SGLangResponse
type SGLangModelData = profile.SGLangModel

// SGLangConverter converts models to SGLang-compatible format with extended metadata
type SGLangConverter struct {
	*BaseConverter
}

// NewSGLangConverter creates a new SGLang format converter
func NewSGLangConverter() ports.ModelResponseConverter {
	return &SGLangConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeSGLang),
	}
}

