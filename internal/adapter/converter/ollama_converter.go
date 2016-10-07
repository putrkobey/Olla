package converter

import (
	"time"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

// OllamaModelResponse represents the Ollama-compatible format response
type OllamaModelResponse struct {
	Models []OllamaModelData `json:"models"`
}

// OllamaModelData represents a single model in Ollama format
type OllamaModelData struct {
	Details    *OllamaDetails `json:"details,omitempty"`
	Name       string         `json:"name"`
	Model      string         `json:"model"`
	ModifiedAt string         `json:"modified_at"`
	Digest     string         `json:"digest"`
	Size       int64          `json:"size"`
}

// OllamaDetails represents model details in Ollama format
type OllamaDetails struct {
	Family            string `json:"family"`
	ParameterSize     string `json:"parameter_size"`
	QuantizationLevel string `json:"quantization_level"`
}

// OllamaConverter converts models to Ollama-compatible format
type OllamaConverter struct {
	*BaseConverter
}

// NewOllamaConverter creates a new Ollama format converter
func NewOllamaConverter() ports.ModelResponseConverter {
	return &OllamaConverter{
		BaseConverter: NewBaseConverter(constants.ProviderTypeOllama),
	}
}

func (c *OllamaConverter) GetFormatName() string {
	return constants.ProviderTypeOllama
}

func (c *OllamaConverter) ConvertToFormat(models []*domain.UnifiedModel, filters ports.ModelFilters) (interface{}, error) {
	filtered := filterModels(models, filters)

	data := make([]OllamaModelData, 0, len(filtered))
	for _, model := range filtered {
		// For Ollama format, use the native Ollama name if available
		ollamaModel := c.convertModel(model)
		if ollamaModel != nil {
