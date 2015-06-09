package translator

import (
	"encoding/json"
	"testing"
)

func TestExtractModelName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		body      []byte
		wantModel string
		wantErr   bool
	}{
		{
			name:      "standard anthropic request",
			body:      []byte(`{"model":"claude-3-opus-20240229","max_tokens":1024,"messages":[{"role":"user","content":"Hello"}]}`),
			wantModel: "claude-3-opus-20240229",
		},
		{
			name:      "model field first",
			body:      []byte(`{"model":"gpt-4","temperature":0.7}`),
			wantModel: "gpt-4",
		},
		{
			name:      "model field last",
			body:      []byte(`{"max_tokens":1024,"messages":[],"model":"llama-3.1-70b"}`),
			wantModel: "llama-3.1-70b",
		},
		{
			name:      "model field in middle of large request",
			body:      []byte(`{"messages":[{"role":"user","content":"Tell me a story"}],"model":"claude-3-haiku-20240307","max_tokens":4096,"stream":true,"temperature":0.5}`),
			wantModel: "claude-3-haiku-20240307",
		},
		{
			name:      "model with slashes and colons",
			body:      []byte(`{"model":"org/repo:latest","max_tokens":100}`),
			wantModel: "org/repo:latest",
		},
		{
			name: "nested model string in content does not match",
			body: []byte(`{"messages":[{"role":"user","content":"{\"model\":\"wrong\"}"}],"model":"correct-model","max_tokens":100}`),
			// gjson extracts the top-level "model" field
			wantModel: "correct-model",
		},
		{
			name:    "missing model field",
			body:    []byte(`{"max_tokens":1024,"messages":[]}`),
			wantErr: true,
		},
		{
			name:    "empty model string",
			body:    []byte(`{"model":"","max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "model as number",
			body:    []byte(`{"model":42,"max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "model as boolean",
			body:    []byte(`{"model":true,"max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "model as array",
			body:    []byte(`{"model":["a","b"],"max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "model as object",
			body:    []byte(`{"model":{"name":"test"},"max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "model as null",
			body:    []byte(`{"model":null,"max_tokens":1024}`),
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			body:    []byte(`{invalid json`),
			wantErr: true,
		},
		{
			name:    "empty body",
			body:    []byte{},
			wantErr: true,
		},
		{
			name:    "nil body",
			body:    nil,
			wantErr: true,
		},
		{
			name:    "just whitespace",
			body:    []byte(`   `),
			wantErr: true,
		},
		{
			name:    "empty JSON object",
			body:    []byte(`{}`),
			wantErr: true,
		},
		{
			name:    "HTML error page",
			body:    []byte(`<html><body>502 Bad Gateway</body></html>`),
