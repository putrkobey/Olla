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
