package converter

import (
	"fmt"
	"strings"

	"github.com/putrkobey/Olla/internal/core/ports"
)

// ConverterFactory creates model response converters
type ConverterFactory struct {
	converters map[string]ports.ModelResponseConverter
}

// NewConverterFactory creates a new converter factory
func NewConverterFactory() *ConverterFactory {
	factory := &ConverterFactory{
		converters: make(map[string]ports.ModelResponseConverter),
	}

	// Register all converters
	factory.RegisterConverter(NewUnifiedConverter())
	factory.RegisterConverter(NewOpenAIConverter())
	factory.RegisterConverter(NewOllamaConverter())
	factory.RegisterConverter(NewLemonadeConverter())
	factory.RegisterConverter(NewLMStudioConverter())
	factory.RegisterConverter(NewLMDeployConverter())
	factory.RegisterConverter(NewSGLangConverter())
	factory.RegisterConverter(NewVLLMConverter())
	factory.RegisterConverter(NewLlamaCppConverter())
	factory.RegisterConverter(NewDockerModelRunnerConverter())
	factory.RegisterConverter(NewVLLMMLXConverter())

	return factory
}

// RegisterConverter registers a new converter
func (f *ConverterFactory) RegisterConverter(converter ports.ModelResponseConverter) {
	f.converters[converter.GetFormatName()] = converter
}
