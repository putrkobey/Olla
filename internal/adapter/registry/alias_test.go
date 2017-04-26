package registry

import (
	"context"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestNewAliasResolver_NilWhenEmpty(t *testing.T) {
	resolver := NewAliasResolver(nil, createTestLogger())
	if resolver != nil {
		t.Error("expected nil resolver for nil aliases")
	}

	resolver = NewAliasResolver(map[string][]string{}, createTestLogger())
	if resolver != nil {
		t.Error("expected nil resolver for empty aliases")
	}
}

func TestNewAliasResolver_Created(t *testing.T) {
	aliases := map[string][]string{
		"my-model": {"model-a", "model-b"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())
	if resolver == nil {
		t.Fatal("expected non-nil resolver")
	}
}

func TestAliasResolver_IsAlias(t *testing.T) {
	aliases := map[string][]string{
		"gpt-oss-120b": {"gpt-oss:120b", "gpt-oss-120b-MLX"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())

	if !resolver.IsAlias("gpt-oss-120b") {
		t.Error("expected gpt-oss-120b to be an alias")
	}
	if resolver.IsAlias("unknown-model") {
		t.Error("expected unknown-model to not be an alias")
	}
	// actual model names are not aliases themselves
	if resolver.IsAlias("gpt-oss:120b") {
		t.Error("expected actual model name gpt-oss:120b to not be an alias")
	}
}

func TestAliasResolver_GetActualModels(t *testing.T) {
	aliases := map[string][]string{
		"gpt-oss-120b": {"gpt-oss:120b", "gpt-oss-120b-MLX", "gguf_gpt_oss_120b.gguf"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())

	models := resolver.GetActualModels("gpt-oss-120b")
	if len(models) != 3 {
		t.Errorf("expected 3 actual models, got %d", len(models))
	}

	models = resolver.GetActualModels("unknown")
	if models != nil {
		t.Error("expected nil for unknown alias")
	}
}

func TestAliasResolver_ResolveEndpoints(t *testing.T) {
	aliases := map[string][]string{
		"gpt-oss-120b": {"gpt-oss:120b", "gpt-oss-120b-MLX", "gguf_gpt_oss_120b.gguf"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())
	ctx := context.Background()

	// Create a registry with some models registered
	reg := NewMemoryModelRegistry(createTestLogger())
	_ = reg.RegisterModel(ctx, "http://ollama:11434", &domain.ModelInfo{Name: "gpt-oss:120b"})
	_ = reg.RegisterModel(ctx, "http://lmstudio:1234", &domain.ModelInfo{Name: "gpt-oss-120b-MLX"})

	endpointToModel, err := resolver.ResolveEndpoints(ctx, "gpt-oss-120b", reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpointToModel) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(endpointToModel))
	}

	if model, ok := endpointToModel["http://ollama:11434"]; !ok || model != "gpt-oss:120b" {
		t.Errorf("expected ollama endpoint to map to gpt-oss:120b, got %q", model)
	}

	if model, ok := endpointToModel["http://lmstudio:1234"]; !ok || model != "gpt-oss-120b-MLX" {
		t.Errorf("expected lmstudio endpoint to map to gpt-oss-120b-MLX, got %q", model)
	}
}

func TestAliasResolver_ResolveEndpoints_NoMatches(t *testing.T) {
	aliases := map[string][]string{
		"gpt-oss-120b": {"model-not-registered"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())
	ctx := context.Background()

	reg := NewMemoryModelRegistry(createTestLogger())

	endpointToModel, err := resolver.ResolveEndpoints(ctx, "gpt-oss-120b", reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpointToModel) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(endpointToModel))
	}
}

func TestAliasResolver_ResolveEndpoints_SameEndpointMultipleModels(t *testing.T) {
	// When the same endpoint has multiple aliased models, the first one found wins
	aliases := map[string][]string{
		"gpt-oss-120b": {"gpt-oss:120b", "gpt-oss-120b"},
	}
	resolver := NewAliasResolver(aliases, createTestLogger())
	ctx := context.Background()

	reg := NewMemoryModelRegistry(createTestLogger())
	_ = reg.RegisterModel(ctx, "http://endpoint:1234", &domain.ModelInfo{Name: "gpt-oss:120b"})
	_ = reg.RegisterModel(ctx, "http://endpoint:1234", &domain.ModelInfo{Name: "gpt-oss-120b"})

	endpointToModel, err := resolver.ResolveEndpoints(ctx, "gpt-oss-120b", reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

