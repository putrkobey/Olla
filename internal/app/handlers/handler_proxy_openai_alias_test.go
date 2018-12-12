package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestFilterEndpointsByProfile_OpenAITypeAlias guards the alias fix in domain.IsCompatibleWith.
// An endpoint configured with type "openai" must be accepted when the request profile's
// SupportedBy slice contains only "openai-compatible". Without the alias, both endpoints
