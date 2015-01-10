package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/config"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/router"
)

// Provider routes come from the profiles alone. This walks every shipped
// profile and every routing prefix, so a provider whose routes need a special
// case in registerProviderPrefixRoutes shows up here rather than in production:
// each prefix must serve its model listing(s), and the catch-all proxy must be
// there for everything else.
func TestRegisterProviderRoutes_AllShippedProfiles(t *testing.T) {
	t.Parallel()

	factory, err := profile.NewFactory("../../../config/profiles")
	require.NoError(t, err, "shipped profiles must load")

	reg := router.NewRouteRegistry(&mockStyledLogger{})
	app := &Application{
		Config:           &config.Config{},
		logger:           &mockStyledLogger{},
		routeRegistry:    reg,
		profileFactory:   factory,
		modelRegistry:    registry.NewUnifiedMemoryModelRegistry(&mockStyledLogger{}, nil, nil, nil),
		repository:       &mockRepository{},
		converterFactory: converter.NewConverterFactory(),
	}
	app.registerProviderRoutes()

	mux := http.NewServeMux()
	reg.WireUp(mux)
	routes := reg.GetRoutes()
