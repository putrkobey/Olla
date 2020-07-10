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
