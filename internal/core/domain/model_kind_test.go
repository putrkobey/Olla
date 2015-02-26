package domain

import "testing"

func TestModelKindFromMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{"nil metadata", nil, ""},
		{"model_type alone", map[string]any{"model_type": "vlm"}, "vlm"},
		{"type alone", map[string]any{"type": "embeddings"}, "embeddings"},
