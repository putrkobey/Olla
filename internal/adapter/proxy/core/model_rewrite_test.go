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

