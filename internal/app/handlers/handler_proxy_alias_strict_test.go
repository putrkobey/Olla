package handlers

import (
	"context"
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

// TestAliasRejection_StrictRouting_FailsFastWithoutProxying is the full-handler
// regression test for #191: a model alias whose real target exists on no endpoint
// must be rejected before the request ever reaches the proxy engine, on both the
// provider-scoped route (providerProxyHandler) and the plain proxy route
// (proxyHandler). Before the fix, resolveAliasEndpoints fell back to returning
// every compatible endpoint, so the request silently proxied to a backend that
// couldn't serve the requested model and returned a false 200.
func TestAliasRejection_StrictRouting_FailsFastWithoutProxying(t *testing.T) {
