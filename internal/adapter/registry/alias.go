package registry

import (
	"context"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/logger"
)

// AliasResolver resolves model aliases to actual model names and their endpoints.
// It sits between the handler and the model registry to transparently expand aliases
// into the concrete model names that backends recognise.
type AliasResolver struct {
	// aliases maps a virtual model name to a list of actual model names
	aliases map[string][]string
	logger  logger.StyledLogger
}

// NewAliasResolver creates a new AliasResolver from the configured alias map.
// Returns nil if no aliases are configured, allowing callers to skip alias logic entirely.
func NewAliasResolver(aliases map[string][]string, logger logger.StyledLogger) *AliasResolver {
	if len(aliases) == 0 {
		return nil
	}

	return &AliasResolver{
		aliases: aliases,
		logger:  logger,
	}
}

// IsAlias returns true if the given model name is a configured alias
func (r *AliasResolver) IsAlias(modelName string) bool {
	_, ok := r.aliases[modelName]
	return ok
}

// GetActualModels returns the list of actual model names for an alias.
// Returns nil if the model name is not an alias.
func (r *AliasResolver) GetActualModels(aliasName string) []string {
	return r.aliases[aliasName]
}

// EndpointModelMapping maps an endpoint URL to the actual model name it should receive
type EndpointModelMapping struct {
	EndpointURL string
