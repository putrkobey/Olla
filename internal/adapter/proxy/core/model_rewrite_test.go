package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
)

func TestRewriteModelForAlias_NoAliasMap(t *testing.T) {
	ctx := context.Background()
	body := `{"model": "original-model", "messages": []}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))

	endpoint := &domain.Endpoint{URLString: "http://localhost:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	// Body should be unchanged
	resultBody, _ := io.ReadAll(r.Body)
	if string(resultBody) != body {
		t.Errorf("body should be unchanged, got %s", string(resultBody))
	}
}

func TestRewriteModelForAlias_EndpointNotInMap(t *testing.T) {
	aliasMap := map[string]string{
		"http://other:1234": "other-model",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	body := `{"model": "alias-name", "messages": []}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))

	endpoint := &domain.Endpoint{URLString: "http://localhost:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	// Body should be unchanged
	resultBody, _ := io.ReadAll(r.Body)
	if string(resultBody) != body {
		t.Errorf("body should be unchanged, got %s", string(resultBody))
	}
}

func TestRewriteModelForAlias_RewritesModel(t *testing.T) {
	aliasMap := map[string]string{
		"http://ollama:11434": "gpt-oss:120b",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	body := `{"model": "gpt-oss-120b", "messages": [{"role": "user", "content": "hello"}]}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")

	endpoint := &domain.Endpoint{URLString: "http://ollama:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	resultBody, _ := io.ReadAll(r.Body)

	var result map[string]json.RawMessage
	if err := json.Unmarshal(resultBody, &result); err != nil {
		t.Fatalf("failed to parse result body: %v", err)
	}

	var modelName string
	if err := json.Unmarshal(result["model"], &modelName); err != nil {
		t.Fatalf("failed to parse model field: %v", err)
	}

	if modelName != "gpt-oss:120b" {
		t.Errorf("expected model to be rewritten to gpt-oss:120b, got %s", modelName)
	}

	// Verify messages are preserved
	if _, ok := result["messages"]; !ok {
		t.Error("messages field should be preserved")
	}
}

func TestRewriteModelForAlias_NilBody(t *testing.T) {
	aliasMap := map[string]string{
		"http://ollama:11434": "gpt-oss:120b",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	r := httptest.NewRequest("GET", "/v1/models", nil)
	r.Body = nil
	r.ContentLength = 0

	endpoint := &domain.Endpoint{URLString: "http://ollama:11434"}
	// Should not panic
	RewriteModelForAlias(ctx, r, endpoint)
}

func TestRewriteModelForAlias_NonJSONBody(t *testing.T) {
	aliasMap := map[string]string{
		"http://ollama:11434": "gpt-oss:120b",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	body := "this is not json"
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))

	endpoint := &domain.Endpoint{URLString: "http://ollama:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	// Body should be unchanged since it's not valid JSON
	resultBody, _ := io.ReadAll(r.Body)
	if string(resultBody) != body {
		t.Errorf("non-JSON body should be unchanged, got %s", string(resultBody))
	}
}

func TestRewriteModelForAlias_NoModelField(t *testing.T) {
	aliasMap := map[string]string{
		"http://ollama:11434": "gpt-oss:120b",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	body := `{"messages": [{"role": "user", "content": "hello"}]}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))

	endpoint := &domain.Endpoint{URLString: "http://ollama:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	// Body should be unchanged since there's no model field
	resultBody, _ := io.ReadAll(r.Body)
	if string(resultBody) != body {
		t.Errorf("body without model field should be unchanged, got %s", string(resultBody))
	}
}

func TestRewriteModelForAlias_UpdatesContentLength(t *testing.T) {
	aliasMap := map[string]string{
		"http://ollama:11434": "a-much-longer-model-name-than-the-original",
	}
	ctx := context.WithValue(context.Background(), constants.ContextModelAliasMapKey, aliasMap)
	body := `{"model": "short"}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(body))
	originalLength := r.ContentLength

	endpoint := &domain.Endpoint{URLString: "http://ollama:11434"}
	RewriteModelForAlias(ctx, r, endpoint)

	resultBody, _ := io.ReadAll(r.Body)
	if r.ContentLength == originalLength {
		t.Error("content length should have been updated")
	}
	if r.ContentLength != int64(len(resultBody)) {
		t.Errorf("content length %d should match body length %d", r.ContentLength, len(resultBody))
	}
}

func TestRewriteModelField(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		newModel  string
		wantModel string
	}{
		{
			name:      "simple replacement",
			body:      `{"model": "old-model"}`,
			newModel:  "new-model",
			wantModel: "new-model",
		},
		{
			name:      "model with colon (ollama format)",
			body:      `{"model": "gpt-oss-120b"}`,
			newModel:  "gpt-oss:120b",
			wantModel: "gpt-oss:120b",
		},
		{
			name:      "preserves other fields",
			body:      `{"model": "old", "temperature": 0.7, "messages": []}`,
			newModel:  "new",
			wantModel: "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteModelField([]byte(tt.body), tt.newModel)

			var parsed map[string]json.RawMessage
			if err := json.Unmarshal(result, &parsed); err != nil {
				t.Fatalf("failed to parse result: %v", err)
			}

			var modelName string
			if err := json.Unmarshal(parsed["model"], &modelName); err != nil {
				t.Fatalf("failed to parse model: %v", err)
			}

			if modelName != tt.wantModel {
				t.Errorf("expected model %q, got %q", tt.wantModel, modelName)
			}
		})
	}
}

func TestRewriteModelField_PreservesKeyOrder(t *testing.T) {
	// The body has keys in a specific order with specific formatting
	body := `{"messages": [{"role": "user", "content": "hello"}], "model": "gpt-oss-120b", "temperature": 0.7, "stream": true}`
	expected := `{"messages": [{"role": "user", "content": "hello"}], "model": "gpt-oss:120b", "temperature": 0.7, "stream": true}`

	result := rewriteModelField([]byte(body), "gpt-oss:120b")

	if string(result) != expected {
		t.Errorf("expected byte-identical output except model name:\n  got:  %s\n  want: %s", string(result), expected)
	}
}

func TestRewriteModelField_PreservesWhitespace(t *testing.T) {
	// Pretty-printed JSON should stay pretty-printed
