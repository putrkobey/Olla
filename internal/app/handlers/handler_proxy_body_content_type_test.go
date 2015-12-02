package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
)

// A JSON body must route by its model whatever Content-Type the client sent. curl -d sends
// form-urlencoded, fetch() with a string body sends text/plain and many scripts send nothing.
// With optimistic/all fallback a model-less request lands on any compatible host, so the
// assertion that only the hosting endpoint is selected catches the header gate.
func TestProxyRouting_NamedModelIgnoresContentType(t *testing.T) {
	t.Parallel()

	styledLog := &mockStyledLogger{}
	profileFactory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err)

	inspectorFactory := inspector.NewFactory(profileFactory, styledLog)
	bodyInspector, err := inspectorFactory.CreateBodyInspector()
	require.NoError(t, err)
	chain := inspectorFactory.CreateChain()
	chain.AddInspector(inspectorFactory.CreatePathInspector())
	chain.AddInspector(bodyInspector)

	const body = `{"model":"nomic-embed-text:latest","input":"hi"}`
	contentTypes := map[string]string{
		"json":            constants.ContentTypeJSON,
		"form-urlencoded": "application/x-www-form-urlencoded",
		"text/plain":      "text/plain;charset=UTF-8",
		"vnd.api+json":    "application/vnd.api+json",
		"x-ndjson":        "application/x-ndjson",
		"missing":         "",
	}

	for name, ct := range contentTypes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			cfg := config.ModelRoutingStrategy{Type: "optimistic", Options: config.ModelRoutingStrategyOptions{FallbackBehavior: constants.FallbackBehaviorAll}}
			unified := registry.NewUnifiedMemoryModelRegistry(styledLog, nil, &cfg, nil)

			fleet := []fleetEndpoint{
				{name: "ollama-has", typ: domain.ProfileOllama, modelsJSON: `{"models":[{"name":"nomic-embed-text:latest"}]}`},
