package registry

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/putrkobey/Olla/internal/core/domain"
)

// Ollama lists a default-tag pull as "name:latest" but clients ask for "name".
// These tests pin the rule: the fallback is exact-first, Ollama-sourced only, and
// never rewrites anything stored.

const (
	ollamaURL  = "http://ollama-1:11434"
	ollama2URL = "http://ollama-2:11434"
	vllmURL    = "http://vllm-1:8000"
)

type tagFixture struct {
	url    string
	name   string
	typ    string
	models []string
}

func newEndpoint(f tagFixture) *domain.Endpoint {
	return &domain.Endpoint{Name: f.name, URLString: f.url, Type: f.typ, Status: domain.StatusHealthy}
}

func modelInfos(names []string) []*domain.ModelInfo {
	out := make([]*domain.ModelInfo, 0, len(names))
	for _, n := range names {
		out = append(out, &domain.ModelInfo{Name: n})
	}
	return out
}

func buildUnified(t *testing.T, fleet ...tagFixture) *UnifiedMemoryModelRegistry {
	t.Helper()
	reg := createTestUnifiedRegistry()
	ctx := context.Background()
	for _, f := range fleet {
		require.NoError(t, reg.RegisterModelsWithEndpoint(ctx, newEndpoint(f), modelInfos(f.models)))
	}
	// Unification is asynchronous. Names resolve case-insensitively, so one source
	// of a merged model would satisfy a bare lookup: wait for every source URL.
	require.Eventually(t, func() bool {
		for _, f := range fleet {
			for _, n := range f.models {
				u, err := reg.GetUnifiedModel(ctx, n)
				if err != nil || u == nil {
					return false
				}
				found := false
				for i := range u.SourceEndpoints {
					found = found || u.SourceEndpoints[i].EndpointURL == f.url
				}
				if !found {
					return false
				}
			}
		}
		return true
	}, 5*time.Second, 5*time.Millisecond)
	return reg
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

func TestUnifiedRegistry_OllamaDefaultTag(t *testing.T) {
	t.Parallel()

	ollama := func(models ...string) tagFixture {
		return tagFixture{url: ollamaURL, name: "ollama-1", typ: domain.ProfileOllama, models: models}
	}
	vllm := func(models ...string) tagFixture {
		return tagFixture{url: vllmURL, name: "vllm-1", typ: domain.ProfileVLLM, models: models}
	}

	tests := []struct {
		name  string
		query string
		want  []string
		fleet []tagFixture
	}{
		{"bare name resolves to ollama latest", "all-minilm", []string{ollamaURL}, []tagFixture{ollama("all-minilm:latest")}},
		{"tagged name is unchanged", "all-minilm:latest", []string{ollamaURL}, []tagFixture{ollama("all-minilm:latest")}},
		{"case differences still resolve through the catalogue", "ALL-MINILM", []string{ollamaURL}, []tagFixture{ollama("all-minilm:latest")}},
		{"a bare name does not reach a non-latest tag", "qwen2.5", nil, []tagFixture{ollama("qwen2.5:0.5b")}},
		{"unknown stays unknown", "nonexistent-xyz", nil, []tagFixture{ollama("all-minilm:latest")}},
		{"vllm-only foo:latest does not answer to foo", "foo", nil, []tagFixture{vllm("foo:latest")}},
		{"vllm bare foo does not answer to foo:latest", "foo:latest", nil, []tagFixture{vllm("foo")}},
		{"exact vllm foo wins over ollama foo:latest", "foo", []string{vllmURL}, []tagFixture{vllm("foo"), ollama("foo:latest")}},
		{"ollama foo:latest still exact beside vllm foo", "foo:latest", []string{ollamaURL}, []tagFixture{vllm("foo"), ollama("foo:latest")}},
		{"shared foo:latest id answers a bare name from ollama only", "foo", []string{ollamaURL}, []tagFixture{vllm("foo:latest"), ollama("foo:latest")}},
		{"shared foo:latest id stays on both when asked exactly", "foo:latest", []string{ollamaURL, vllmURL}, []tagFixture{vllm("foo:latest"), ollama("foo:latest")}},
		{"registry host with a port is not a tag", "registry.example.com:5000/team/foo", []string{ollamaURL}, []tagFixture{ollama("registry.example.com:5000/team/foo:latest")}},
		{
			"two ollama hosts both answer", "foo", []string{ollamaURL, ollama2URL},
			[]tagFixture{ollama("foo:latest"), {url: ollama2URL, name: "ollama-2", typ: domain.ProfileOllama, models: []string{"foo:latest"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reg := buildUnified(t, tt.fleet...)
			ctx := context.Background()

			got, err := reg.GetEndpointsForModel(ctx, tt.query)
			require.NoError(t, err)
			assert.Equal(t, sorted(tt.want), sorted(got))
			assert.Equal(t, len(tt.want) > 0, reg.IsModelAvailable(ctx, tt.query))
		})
	}
}

func TestUnifiedRegistry_OllamaDefaultTag_GetUnifiedModel(t *testing.T) {
	t.Parallel()

	reg := buildUnified(t,
		tagFixture{url: ollamaURL, name: "ollama-1", typ: domain.ProfileOllama, models: []string{"all-minilm:latest"}},
		tagFixture{url: vllmURL, name: "vllm-1", typ: domain.ProfileVLLM, models: []string{"only-vllm:latest"}},
