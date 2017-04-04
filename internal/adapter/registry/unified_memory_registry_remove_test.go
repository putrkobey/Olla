package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/adapter/unifier"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// The unifier's own store answers alias lookups once the direct ID lookup misses, so
// removing an endpoint has to clear it there too or a removed host keeps being offered.
func TestUnifiedRegistry_RemoveEndpointForgetsItInAliasResolution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("sole host", func(t *testing.T) {
		t.Parallel()
		r := NewUnifiedMemoryModelRegistry(createTestLogger(), nil, nil, nil)
		require.NoError(t, r.RegisterModelsWithEndpoint(ctx, raceEndpoint("a", "http://a:11434"),
			[]*domain.ModelInfo{raceModel("solo:1b", "sha-solo")}))
		r.WaitForUnification()

		before, err := r.GetUnifiedModel(ctx, "solo:1b")
		require.NoError(t, err)
		require.Len(t, before.SourceEndpoints, 1)

		require.NoError(t, r.RemoveEndpoint(ctx, "http://a:11434"))

		if m, err := r.GetUnifiedModel(ctx, "solo:1b"); err == nil {
			t.Errorf("model must be gone, still resolves with endpoints %v", m.SourceEndpoints)
		}
		eps, _ := r.GetEndpointsForModel(ctx, "solo:1b")
		assert.Empty(t, eps, "a removed endpoint must not be offered")
	})

	t.Run("one of two hosts", func(t *testing.T) {
		t.Parallel()
		r := NewUnifiedMemoryModelRegistry(createTestLogger(), nil, nil, nil)
		seedTwoEndpoints(t, r)

		require.NoError(t, r.RemoveEndpoint(ctx, "http://a:11434"))

		for _, name := range []string{"llama3:8b", "llama3-8b-b"} {
			m, err := r.GetUnifiedModel(ctx, name)
			require.NoError(t, err, name)
			urls := make([]string, 0, len(m.SourceEndpoints))
			for _, s := range m.SourceEndpoints {
				urls = append(urls, s.EndpointURL)
			}
			assert.Equal(t, []string{"http://b:11434"}, urls, "%s must list only the surviving endpoint", name)

			eps, err := r.GetEndpointsForModel(ctx, name)
			require.NoError(t, err, name)
			assert.Equal(t, []string{"http://b:11434"}, eps, "GetEndpointsForModel(%s)", name)
		}
	})
}

func seedLifecycleRegistry(t *testing.T) (*LifecycleUnifiedRegistry, *unifier.LifecycleUnifier) {
	t.Helper()
	ctx := context.Background()
	r := NewLifecycleUnifiedRegistry(createTestLogger(), unifier.DefaultConfig())
	require.NoError(t, r.Start(ctx))
	t.Cleanup(func() { _ = r.Stop(ctx) })

	require.NoError(t, r.RegisterModelsWithEndpoint(ctx, raceEndpoint("a", "http://a:11434"),
		[]*domain.ModelInfo{raceModel("solo:1b", "sha-solo")}))
	r.WaitForUnification()

	lu, ok := r.unifier.(*unifier.LifecycleUnifier)
	require.True(t, ok)
	return r, lu
}

func TestLifecycleUnifiedRegistry_RegisterThenRemoveRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	r, _ := seedLifecycleRegistry(t)

	_, err := r.GetUnifiedModel(ctx, "solo:1b")
	require.NoError(t, err)
