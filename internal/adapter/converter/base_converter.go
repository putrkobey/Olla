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
