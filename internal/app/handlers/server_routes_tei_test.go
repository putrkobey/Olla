package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/inspector"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"github.com/putrkobey/Olla/internal/logger"
	"github.com/putrkobey/Olla/internal/router"
)

// TestRegisterProviderRoutes_TextEmbeddingsInference exercises the real
// profile-driven route registration for TEI. It guards two things the static
// fallback cannot: both routing prefixes (text-embeddings-inference and tei)
// produce routes, and /info - TEI's discovery path, which carries model
// metadata rather than a listing - is NOT intercepted: it falls through to
// the catch-all proxy and is forwarded verbatim. The OpenAI model listing is
// served at /v1/models instead.
func TestRegisterProviderRoutes_TextEmbeddingsInference(t *testing.T) {
	t.Parallel()

	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err, "shipped profiles must load")

	teiURL, _ := url.Parse("http://tei:8080")
	endpoint := &domain.Endpoint{
		Name:      "tei-1",
		URL:       teiURL,
		URLString: teiURL.String(),
		Status:    domain.StatusHealthy,
		Type:      constants.ProviderTypeTextEmbeddingsInference,
	}

	unifiedRegistry := registry.NewUnifiedMemoryModelRegistry(&mockStyledLogger{}, nil, nil, nil)
	ctx := context.Background()
	unifiedRegistry.RegisterEndpoint(endpoint)

	const teiModel = "Snowflake/snowflake-arctic-embed-l-v2.0"
	require.NoError(t, unifiedRegistry.RegisterModelsWithEndpoint(ctx, endpoint, []*domain.ModelInfo{{Name: teiModel}}))

	require.Eventually(t, func() bool {
		models, err := unifiedRegistry.GetUnifiedModels(ctx)
		return err == nil && len(models) >= 1
	}, 5*time.Second, 10*time.Millisecond, "async unification did not complete")

	reg := router.NewRouteRegistry(&mockStyledLogger{})
	app := &Application{
		Config:           &config.Config{},
		logger:           &mockStyledLogger{},
		routeRegistry:    reg,
		profileFactory:   factory,
		modelRegistry:    unifiedRegistry,
		repository:       &mockRepository{endpoints: []*domain.Endpoint{endpoint}},
		converterFactory: converter.NewConverterFactory(),
	}

	app.registerProviderRoutes()

	// Both prefixes must expose the OpenAI listing, and /info must not be
	// intercepted (it is forwarded verbatim by the catch-all proxy).
	for _, prefix := range []string{constants.ProviderPrefixTextEmbeddingsInference1, constants.ProviderPrefixTextEmbeddingsInference2} {
		routes := reg.GetRoutes()
		assert.NotContains(t, routes, "/olla/"+prefix+"/info", "prefix %s must not intercept /info", prefix)
