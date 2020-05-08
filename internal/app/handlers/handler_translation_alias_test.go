package handlers

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestPrepareProxyContext_InjectsAliasMapWhenPresent(t *testing.T) {
	t.Parallel()

	app := &Application{}

	profile := domain.NewRequestProfile("/v1/messages")
	aliasMap := map[string]string{
		"http://ollama:11434":  "gpt-oss:120b",
		"http://lmstudio:1234": "gpt-oss-120b-MLX",
	}
	profile.SetInspectionMeta(constants.ContextModelAliasMapKey, aliasMap)
	profile.RoutingDecision = &domain.ModelRoutingDecision{
		Strategy: "alias",
		Action:   "routed",
	}

	pr := &proxyRequest{
		model:   "gpt-oss-120b",
		profile: profile,
		stats:   &ports.RequestStats{},
