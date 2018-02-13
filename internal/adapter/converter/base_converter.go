package converter

import (
	"strings"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// Known AI model organisations for owner detection
// Package-level to avoid allocations on every call
// TODO: Move to TaxonomyService + Files
var knownOrganizations = []string{
	"openai", "anthropic", "google", "meta", "microsoft", "amazon", "xai", "apple",
	"cohere", "ai21", "databricks", "snowflake", "ibm", "salesforce", "nvidia",

	"mistral", "qwen", "alibaba", "deepseek", "zhipuai", "tii", // qwen/alibaba, glm/zhipuai, falcon/tii
	"01ai", "baichuan", "stability", "eleutherai", "mosaicml", "bigcode",
	"servicenow", "cerebras", "nomic", "huggingface",

	"openrouter", "together", "replicate", "fireworks", "perplexity", "groq",
}

// BaseConverter provides common functionality for all converters
type BaseConverter struct {
	providerType string
}

// NewBaseConverter creates a new base converter
func NewBaseConverter(providerType string) *BaseConverter {
	return &BaseConverter{
		providerType: providerType,
	}
}

// FindProviderAlias finds the provider-specific alias for a unified model
func (b *BaseConverter) FindProviderAlias(model *domain.UnifiedModel) (string, bool) {
	for _, alias := range model.Aliases {
		if alias.Source == b.providerType {
			return alias.Name, true
		}
	}
	return "", false
}

// FindNativeName looks up the provider-native model name from aliases, or ""
// when none is set. Each converter used to carry its own one-line wrapper
// around FindProviderAlias for this (findVLLMNativeName, findLlamaCppNativeName,
// etc.) - all identical, just discarding the found bool.
func (b *BaseConverter) FindNativeName(model *domain.UnifiedModel) string {
	if alias, found := b.FindProviderAlias(model); found {
		return alias
	}
	return ""
}

// FindProviderEndpoint finds the provider-specific endpoint for a unified model
func (b *BaseConverter) FindProviderEndpoint(model *domain.UnifiedModel, providerName string) *domain.SourceEndpoint {
	for i := range model.SourceEndpoints {
		ep := &model.SourceEndpoints[i]
		if providerName != "" && ep.NativeName == providerName {
			return ep
		}
	}
	return nil
}

// ExtractMetadataString safely extracts a string value from metadata
func (b *BaseConverter) ExtractMetadataString(metadata map[string]interface{}, key string) string {
	if val, ok := metadata[key].(string); ok {
		return val
	}
	return ""
}

// ExtractMetadataInt safely extracts an int value from metadata
func (b *BaseConverter) ExtractMetadataInt(metadata map[string]interface{}, key string) int {
	if val, ok := metadata[key].(int); ok {
		return val
	}
	if val, ok := metadata[key].(float64); ok {
		return int(val)
	}
	return 0
}

// ExtractMetadataBool safely extracts a bool value from metadata
func (b *BaseConverter) ExtractMetadataBool(metadata map[string]interface{}, key string) bool {
	if val, ok := metadata[key].(bool); ok {
		return val
	}
	return false
}

// GetEndpointDiskSize returns the disk size from endpoint or model
func (b *BaseConverter) GetEndpointDiskSize(model *domain.UnifiedModel, endpoint *domain.SourceEndpoint) int64 {
	if endpoint != nil && endpoint.DiskSize > 0 {
		return endpoint.DiskSize
	}
	return model.DiskSize
}

// DetermineModelState determines the model state from endpoint
func (b *BaseConverter) DetermineModelState(endpoint *domain.SourceEndpoint, defaultState string) string {
	if endpoint != nil && endpoint.State != "" {
		return endpoint.State
	}
	return defaultState
}

// HasCapability checks if a model has a specific capability
func (b *BaseConverter) HasCapability(capabilities []string, capability string) bool {
	return hasCapability(capabilities, capability)
}

// DetermineModelType determines the model type based on capabilities and metadata
func (b *BaseConverter) DetermineModelType(model *domain.UnifiedModel, defaultType string) string {
	// Check metadata first; the specific kind beats the generic "model" marker
	if modelType := domain.ModelKindFromMetadata(model.Metadata); modelType != "" {
		return modelType
	}

	// Check capabilities
	if b.HasCapability(model.Capabilities, "vision") {
		return "vlm"
	}
	if b.HasCapability(model.Capabilities, "embedding") || b.HasCapability(model.Capabilities, "embeddings") {
		return "embeddings"
	}
	// backends that report no kind (vLLM) get rerank and classification from
	// the model name, so the capability is all there is to type them by
	if b.HasCapability(model.Capabilities, "rerank") {
		return "reranker"
	}
	if b.HasCapability(model.Capabilities, "classification") {
		return "classifier"
	}

	return defaultType
}

// ModelKind is the model kind (llm, vlm, embeddings, reranker, classifier) the
// API reports for a unified model. A model whose backend names no kind and
// whose capabilities say nothing else but text generation is an llm; anything
// else unclassifiable reports "" so the field is omitted rather than guessed.
func ModelKind(model *domain.UnifiedModel) string {
	base := NewBaseConverter("")
	if kind := base.DetermineModelType(model, ""); kind != "" {
		return kind
	}
	if base.HasCapability(model.Capabilities, "text-generation") || base.HasCapability(model.Capabilities, "chat") ||
		base.HasCapability(model.Capabilities, "completion") {
		return "llm"
	}
	return ""
}

// ConversionHelper groups all helper methods for use in concrete converters
type ConversionHelper struct {
	*BaseConverter
	Model    *domain.UnifiedModel
	Endpoint *domain.SourceEndpoint
	Alias    string
}

// NewConversionHelper creates a helper for converting a specific model
func (b *BaseConverter) NewConversionHelper(model *domain.UnifiedModel) *ConversionHelper {
	alias, _ := b.FindProviderAlias(model)
	endpoint := b.FindProviderEndpoint(model, alias)

	return &ConversionHelper{
		BaseConverter: b,
		Model:         model,
		Alias:         alias,
		Endpoint:      endpoint,
	}
}

// ShouldSkip returns true if this model should be skipped for this provider
func (h *ConversionHelper) ShouldSkip() bool {
	return h.Alias == ""
}

// GetDiskSize returns the appropriate disk size for the model
func (h *ConversionHelper) GetDiskSize() int64 {
	return h.GetEndpointDiskSize(h.Model, h.Endpoint)
}

