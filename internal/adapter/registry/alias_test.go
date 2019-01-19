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
