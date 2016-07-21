package unifier

import (
	"testing"

	"github.com/putrkobey/Olla/internal/core/constants"
)

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		metadata     map[string]interface{}
		endpointType string
		want         string
	}{
		{
			name:         "metadata platform hint",
			metadata:     map[string]interface{}{"platform": "CUSTOM"},
			endpointType: "lmstudio",
			want:         "custom",
		},
		{
			name:         "ollama version in metadata",
			metadata:     map[string]interface{}{"ollama.version": "0.1.0"},
			endpointType: "lmstudio",
			want:         constants.ProviderTypeOllama,
		},
		{
			name:         "lmstudio version in metadata",
			metadata:     map[string]interface{}{"lmstudio.version": "1.0.0"},
			endpointType: "openai",
			want:         constants.ProviderTypeLMStudio,
		},
		{
			name:         "endpoint type used when no metadata hints",
			metadata:     map[string]interface{}{},
			endpointType: "lmstudio",
			want:         "lmstudio",
		},
		{
			name:         "endpoint type lowercase",
			metadata:     map[string]interface{}{},
			endpointType: "LMStudio",
			want:         "lmstudio",
		},
