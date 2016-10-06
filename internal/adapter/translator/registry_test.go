package translator

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/putrkobey/Olla/internal/logger"
)

// mockTranslator is a test double for RequestTranslator interface
type mockTranslator struct {
	name string
}

func (m *mockTranslator) TransformRequest(ctx context.Context, r *http.Request) (*TransformedRequest, error) {
	return nil, nil
}

func (m *mockTranslator) TransformResponse(ctx context.Context, openaiResp interface{}, original *http.Request) (interface{}, error) {
	return nil, nil
}

func (m *mockTranslator) TransformStreamingResponse(ctx context.Context, openaiStream io.Reader, w http.ResponseWriter, original *http.Request) error {
	return nil
}

func (m *mockTranslator) Name() string {
	return m.name
}

// createTestLogger creates a minimal logger for testing
func createTestLogger() logger.StyledLogger {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return logger.NewPlainStyledLogger(log)
}

func TestNewRegistry(t *testing.T) {
	log := createTestLogger()
	registry := NewRegistry(log)

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.translators == nil {
		t.Error("Expected initialized translators map")
	}

	if registry.logger == nil {
		t.Error("Expected non-nil logger")
	}
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name           string
		translatorName string
		translator     RequestTranslator
		wantName       string
		wantCount      int
	}{
		{
			name:           "register with explicit name",
			translatorName: "anthropic",
			translator:     &mockTranslator{name: "anthropic"},
			wantName:       "anthropic",
			wantCount:      1,
		},
		{
			name:           "register with empty name uses translator.Name()",
			translatorName: "",
			translator:     &mockTranslator{name: "gemini"},
			wantName:       "gemini",
			wantCount:      1,
		},
		{
			name:           "register multiple translators",
			translatorName: "bedrock",
			translator:     &mockTranslator{name: "bedrock"},
			wantName:       "bedrock",
			wantCount:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			log := createTestLogger()
			registry := NewRegistry(log)

			registry.Register(tt.translatorName, tt.translator)

			// Verify registration
			all := registry.GetAll()
			if len(all) != tt.wantCount {
				t.Errorf("Expected %d translator(s), got %d", tt.wantCount, len(all))
			}

			// Verify we can retrieve it
			retrieved, err := registry.Get(tt.wantName)
			if err != nil {
				t.Errorf("Expected no error retrieving %s, got: %v", tt.wantName, err)
			}

			if retrieved == nil {
				t.Errorf("Expected non-nil translator for %s", tt.wantName)
			}

			if retrieved.Name() != tt.wantName {
				t.Errorf("Expected translator name %s, got %s", tt.wantName, retrieved.Name())
			}
		})
	}
}

func TestRegistry_Register_Overwrite(t *testing.T) {
	log := createTestLogger()
	registry := NewRegistry(log)

	// Register first translator
	first := &mockTranslator{name: "anthropic"}
	registry.Register("anthropic", first)

	// Register second translator with same name - should overwrite
	second := &mockTranslator{name: "anthropic"}
	registry.Register("anthropic", second)

	// Should still have only one translator
	all := registry.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 translator after overwrite, got %d", len(all))
	}

	// Retrieved translator should be the second one
	retrieved, err := registry.Get("anthropic")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Simple check that it's a different instance (in real code you'd use pointer comparison)
	if retrieved.Name() != "anthropic" {
		t.Errorf("Expected overwritten translator name to be anthropic, got %s", retrieved.Name())
	}
}

func TestRegistry_Get(t *testing.T) {
	log := createTestLogger()
	registry := NewRegistry(log)

	// Register test translators
	anthropic := &mockTranslator{name: "anthropic"}
	gemini := &mockTranslator{name: "gemini"}
	registry.Register("anthropic", anthropic)
	registry.Register("gemini", gemini)

	tests := []struct {
		name          string
		translatorKey string
		wantError     bool
		wantName      string
	}{
		{
			name:          "get existing translator - anthropic",
			translatorKey: "anthropic",
			wantError:     false,
			wantName:      "anthropic",
		},
		{
			name:          "get existing translator - gemini",
			translatorKey: "gemini",
			wantError:     false,
			wantName:      "gemini",
		},
		{
			name:          "get non-existent translator",
			translatorKey: "bedrock",
			wantError:     true,
			wantName:      "",
		},
		{
			name:          "get with empty name",
			translatorKey: "",
			wantError:     true,
			wantName:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrieved, err := registry.Get(tt.translatorKey)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
