package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestFilterEndpointsByProfile_OpenAITypeAlias guards the alias fix in domain.IsCompatibleWith.
// An endpoint configured with type "openai" must be accepted when the request profile's
// SupportedBy slice contains only "openai-compatible". Without the alias, both endpoints
// would be dropped and the fallback warning fires — the original symptom of issue #148.
func TestFilterEndpointsByProfile_OpenAITypeAlias(t *testing.T) {
	t.Parallel()

	styledLog := &mockStyledLogger{}

	openaiEndpoint := &domain.Endpoint{
		Name:      "openai-backend",
		URLString: "http://openai-backend:8080",
		Type:      "openai",
		Status:    domain.StatusHealthy,
	}
	compatibleEndpoint := &domain.Endpoint{
