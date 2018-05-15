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
