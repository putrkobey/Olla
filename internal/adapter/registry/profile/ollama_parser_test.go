package profile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaParser_CapabilitiesTyping(t *testing.T) {
	t.Parallel()

	parser := &ollamaParser{}

	tests := []struct {
		name     string
		body     string
		wantType string // empty means Type must be nil
		wantCaps []string
	}{
		{
			name:     "embedding only is typed embeddings",
			body:     `{"models":[{"name":"all-minilm:latest","details":{"family":"bert"},"capabilities":["embedding"]}]}`,
			wantType: "embeddings",
		},
		{
			name:     "completion vision tools is a vlm with vision and function calling",
			body:     `{"models":[{"name":"qwen3-vl:8b","details":{"family":"qwen3vl"},"capabilities":["completion","vision","tools","thinking"]}]}`,
			wantType: "vlm",
			wantCaps: []string{"vision", "function_calling"},
		},
		{
			name:     "completion plus vision is a generative vlm",
			body:     `{"models":[{"name":"llava:13b","capabilities":["completion","vision"]}]}`,
			wantType: "vlm",
			wantCaps: []string{"vision"},
		},
		{
			name:     "vision without a completion entry is still a vlm",
			body:     `{"models":[{"name":"vision-only:latest","capabilities":["vision"]}]}`,
			wantType: "vlm",
			wantCaps: []string{"vision"},
		},
		{
			name:     "completion plus tools has the capability but no kind",
			body:     `{"models":[{"name":"llama3.1:8b","capabilities":["completion","tools"]}]}`,
			wantCaps: []string{"function_calling"},
		},
		{
			name:     "embedding plus vision stays embedding only",
			body:     `{"models":[{"name":"odd:latest","capabilities":["embedding","vision"]}]}`,
			wantType: "embeddings",
		},
		{
			name: "missing capabilities (older Ollama) stays untyped",
			body: `{"models":[{"name":"llama3:latest","digest":"abc","details":{"family":"llama"}}]}`,
		},
		{
			name: "embedding plus completion stays untyped",
			body: `{"models":[{"name":"hybrid:latest","details":{"family":"x"},"capabilities":["embedding","completion"]}]}`,
		},
		{
			name:     "embedding plus unknown value is still embedding only",
			body:     `{"models":[{"name":"rerank:latest","capabilities":["embedding","rerank"]}]}`,
			wantType: "embeddings",
		},
