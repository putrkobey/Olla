package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/converter"
	"github.com/putrkobey/Olla/internal/adapter/registry"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Every handler that serialises the unified view reads *UnifiedModel pointers
// the registry published without a lock. This loops that route table while
// discovery merges a second endpoint, removes it and re-registers, so a handler
// that reads Aliases, SourceEndpoints or Metadata mid-mutation fails under -race
// (or crashes the run with a map fatal without it).
func TestUnifiedViewHandlers_ConcurrentWithRegistration(t *testing.T) {
	t.Parallel()

	newEndpoint := func(name string) *domain.Endpoint {
		u, err := url.Parse("http://" + name + ":11434")
		require.NoError(t, err)
		return &domain.Endpoint{Name: name, URL: u, URLString: u.String(), Type: domain.ProfileOllama, Status: domain.StatusHealthy}
	}
	model := func(name string) []*domain.ModelInfo {
		digest := "sha256:shared"
		return []*domain.ModelInfo{{Name: name, Size: 100, Details: &domain.ModelDetails{Digest: &digest}}}
	}

	epA, epB := newEndpoint("a"), newEndpoint("b")
	ctx := context.Background()

	for range 30 {
		reg := registry.NewUnifiedMemoryModelRegistry(&mockStyledLogger{}, nil, nil, nil)
		reg.RegisterEndpoint(epA)
		reg.RegisterEndpoint(epB)
		require.NoError(t, reg.RegisterModelsWithEndpoint(ctx, epA, model("llama3:8b")))
		reg.WaitForUnification()

		app := &Application{
