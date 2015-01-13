package inspector

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
)

func TestNewChain(t *testing.T) {
	logger := createTestLogger()
	chain := NewChain(logger)

	if chain == nil {
		t.Fatal("NewChain() should not return nil")
	}

	if len(chain.inspectors) != 0 {
		t.Errorf("NewChain() should start with empty inspectors, got %d", len(chain.inspectors))
	}
}

func TestChain_AddInspector(t *testing.T) {
	logger := createTestLogger()
	chain := NewChain(logger)

	inspector1 := &mockInspector{name: "test1"}
	inspector2 := &mockInspector{name: "test2"}

	chain.AddInspector(inspector1)
	if len(chain.inspectors) != 1 {
		t.Errorf("AddInspector() should add inspector, got %d inspectors", len(chain.inspectors))
	}

	chain.AddInspector(inspector2)
	if len(chain.inspectors) != 2 {
		t.Errorf("AddInspector() should add second inspector, got %d inspectors", len(chain.inspectors))
	}

	if chain.inspectors[0] != inspector1 {
		t.Error("AddInspector() should maintain order")
	}
	if chain.inspectors[1] != inspector2 {
		t.Error("AddInspector() should maintain order")
	}
}

func TestChain_Inspect_EmptyChain(t *testing.T) {
	logger := createTestLogger()
	chain := NewChain(logger)
	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	targetPath := "/v1/chat/completions"

	profile, err := chain.Inspect(ctx, req, targetPath)

	if err != nil {
		t.Errorf("Inspect() with empty chain should not error, got %v", err)
	}

	if profile == nil {
		t.Fatal("Inspect() should return a profile, got nil")
	}

	if profile.Path != targetPath {
		t.Errorf("Inspect() profile path = %v, want %v", profile.Path, targetPath)
	}

	if len(profile.SupportedBy) != 0 {
		t.Errorf("Inspect() with empty chain should have no supported profiles, got %v", profile.SupportedBy)
	}
}

func TestChain_Inspect_SingleInspector_Success(t *testing.T) {
	logger := createTestLogger()
	chain := NewChain(logger)
	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	targetPath := "/v1/chat/completions"

	inspector := &mockInspector{
		name: "test",
		inspectFunc: func(ctx context.Context, r *http.Request, profile *domain.RequestProfile) error {
			profile.AddSupportedProfile("test-profile")
			profile.SetInspectionMeta("test-key", "test-value")
			return nil
		},
	}

	chain.AddInspector(inspector)
	profile, err := chain.Inspect(ctx, req, targetPath)

	if err != nil {
		t.Errorf("Inspect() should not error, got %v", err)
	}

	if !contains(profile.SupportedBy, "test-profile") {
		t.Errorf("Inspect() should include test-profile, got %v", profile.SupportedBy)
	}

	value, exists := profile.InspectionMeta.Load("test-key")
	if !exists || value != "test-value" {
		t.Errorf("Inspect() should set metadata, got %v", value)
	}

	if !inspector.called {
		t.Error("Inspect() should call inspector")
	}
}

func TestChain_Inspect_MultipleInspectors_AllSuccess(t *testing.T) {
	logger := createTestLogger()
	chain := NewChain(logger)
	ctx := context.Background()
	req, _ := http.NewRequest("POST", "/v1/chat/completions", nil)
	targetPath := "/v1/chat/completions"

	inspector1 := &mockInspector{
		name: "inspector1",
		inspectFunc: func(ctx context.Context, r *http.Request, profile *domain.RequestProfile) error {
			profile.AddSupportedProfile("profile1")
			return nil
		},
	}

	inspector2 := &mockInspector{
		name: "inspector2",
		inspectFunc: func(ctx context.Context, r *http.Request, profile *domain.RequestProfile) error {
			profile.AddSupportedProfile("profile2")
			profile.SetInspectionMeta("inspector2", "data")
			return nil
		},
	}

	chain.AddInspector(inspector1)
	chain.AddInspector(inspector2)

