package inspector

import (
	"context"
	"net/http"
	"testing"

	"github.com/putrkobey/Olla/internal/adapter/registry/profile"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestNewFactory(t *testing.T) {
	profileFactory := createTestProfileFactory(t)
	logger := createTestLogger()

	factory := NewFactory(profileFactory, logger)

	if factory == nil {
		t.Fatal("NewFactory() should not return nil")
	}

	if factory.profileFactory != profileFactory {
		t.Error("NewFactory() should store profile factory reference")
	}

	if factory.logger != logger {
		t.Error("NewFactory() should store logger reference")
	}
}

func TestFactory_CreateDefaultChain(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	chain := factory.CreateDefaultChain()

	if chain == nil {
		t.Fatal("CreateDefaultChain() should not return nil")
	}

	// chain is functional by testing with a known path
	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	targetPath := "/v1/chat/completions"

	profile, err := chain.Inspect(ctx, req, targetPath)

	if err != nil {
		t.Errorf("CreateDefaultChain() chain should work, got error: %v", err)
	}

	if profile == nil {
		t.Fatal("CreateDefaultChain() chain should return profile")
	}

	if len(profile.SupportedBy) == 0 {
		t.Error("CreateDefaultChain() should create functional chain that identifies supported profiles")
	}

	// should include all major profiles for chat completions
	expectedProfiles := []string{domain.ProfileOllama, domain.ProfileLmStudio, domain.ProfileOpenAICompatible}
	for _, expected := range expectedProfiles {
		if !contains(profile.SupportedBy, expected) {
			t.Errorf("CreateDefaultChain() should support %v for chat completions, got %v", expected, profile.SupportedBy)
		}
	}
}

func TestFactory_CreateDefaultChain_PathInspectorIncluded(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	chain := factory.CreateDefaultChain()

	// test with Ollama-specific path
	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/api/generate", nil)
	targetPath := "/api/generate"

	profile, err := chain.Inspect(ctx, req, targetPath)

	if err != nil {
		t.Errorf("CreateDefaultChain() error = %v, want nil", err)
	}

	if !contains(profile.SupportedBy, domain.ProfileOllama) {
		t.Errorf("CreateDefaultChain() should include PathInspector that supports Ollama for /api/generate")
	}

	if contains(profile.SupportedBy, domain.ProfileLmStudio) {
		t.Errorf("CreateDefaultChain() PathInspector should not support LMStudio for Ollama-specific path")
	}
}

func TestFactory_CreateDefaultChain_MultipleCallsReturnDifferentInstances(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	// 2Chains!
	chain1 := factory.CreateDefaultChain()
	chain2 := factory.CreateDefaultChain()

	if chain1 == chain2 {
		t.Error("CreateDefaultChain() should return different instances on each call")
	}

	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	targetPath := "/v1/chat/completions"

	profile1, err1 := chain1.Inspect(ctx, req, targetPath)
	profile2, err2 := chain2.Inspect(ctx, req, targetPath)

	if err1 != nil || err2 != nil {
		t.Errorf("Both chains should be functional, got errors: %v, %v", err1, err2)
	}

	if len(profile1.SupportedBy) == 0 || len(profile2.SupportedBy) == 0 {
		t.Error("Both chains should produce results")
	}
}

func TestFactory_CreatePathInspector(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	inspector := factory.CreatePathInspector()

	if inspector == nil {
		t.Fatal("CreatePathInspector() should not return nil")
	}

	if inspector.Name() != "path" {
		t.Errorf("CreatePathInspector() name = %v, want 'path'", inspector.Name())
	}

	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	profile := domain.NewRequestProfile("/v1/chat/completions")

	err := inspector.Inspect(ctx, req, profile)

	if err != nil {
		t.Errorf("CreatePathInspector() inspector should work, got error: %v", err)
	}

	if len(profile.SupportedBy) == 0 {
		t.Error("CreatePathInspector() should create functional path inspector")
	}
}

func TestFactory_CreatePathInspector_MultipleCalls(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	inspector1 := factory.CreatePathInspector()
	inspector2 := factory.CreatePathInspector()

	if inspector1 == inspector2 {
		t.Error("CreatePathInspector() should return different instances")
	}

	if inspector1.Name() != inspector2.Name() {
		t.Error("CreatePathInspector() instances should have same name")
	}
}

func TestFactory_CreateChain(t *testing.T) {
	logger := createTestLogger()
	profileFactory := createTestProfileFactory(t)
	factory := NewFactory(profileFactory, logger)

	chain := factory.CreateChain()

	if chain == nil {
		t.Fatal("CreateChain() should not return nil")
