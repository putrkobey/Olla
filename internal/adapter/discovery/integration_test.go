package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// TestEnhancedModelDiscoveryIntegration tests the detection of various model data
func TestEnhancedModelDiscoveryIntegration(t *testing.T) {
	tests := []struct {
		name               string
		platformType       string
		response           string
		expectedModelCount int
		validateMetadata   func(*testing.T, []*domain.ModelInfo)
	}{
		{
			name:               "Ollama Platform Rich Metadata",
			platformType:       domain.ProfileOllama,
			expectedModelCount: 2,
			response: `{
				"models": [
					{
						"name": "devstral:latest",
						"size": 14333927918,
						"digest": "c4b2fa0c33d75457e5f1c8507c906a79e73285768686db13b9cbac0c7ee3a854",
						"modified_at": "2025-05-30T14:24:44.5116551+10:00",
						"description": "Code-focused model for development tasks",
						"details": {
							"parameter_size": "23.6B",
							"quantization_level": "Q4_K_M",
							"family": "llama",
							"families": ["llama"],
							"format": "gguf",
							"parent_model": "codestral"
						}
					},
					{
						"name": "codegemma:9b",
						"size": 5011852809,
						"description": "Code generation model",
						"digest": "sha256:0c96700aaada572ce9bb6999d1fda9b53e9e6cef5d74fda1e066a1ba811b93f3",
						"details": {
							"format": "gguf",
							"family": "gemma",
							"families": ["gemma"],
							"parameter_size": "9B",
							"quantization_level": "Q4_0"
						}
					}
				]
			}`,
			validateMetadata: func(t *testing.T, models []*domain.ModelInfo) {
				var devstral *domain.ModelInfo
				for _, model := range models {
					if model.Name == "devstral:latest" {
						devstral = model
						break
					}
				}

				if devstral == nil {
					t.Fatal("Expected to find devstral:latest model")
				}

				if devstral.Size != 14333927918 {
					t.Errorf("Expected size 14333927918, got %d", devstral.Size)
				}

				if devstral.Description != "Code-focused model for development tasks" {
					t.Errorf("Expected description to be preserved, got %q", devstral.Description)
				}

				if devstral.Details == nil {
					t.Fatal("Expected devstral model to have details")
				}

				if devstral.Details.ParameterSize == nil || *devstral.Details.ParameterSize != "23.6B" {
					t.Error("Expected parameter_size to be '23.6B'")
				}

				if devstral.Details.QuantizationLevel == nil || *devstral.Details.QuantizationLevel != "Q4_K_M" {
					t.Error("Expected quantization_level to be 'Q4_K_M'")
				}

				if devstral.Details.Family == nil || *devstral.Details.Family != "llama" {
					t.Error("Expected family to be 'llama'")
				}

				if devstral.Details.Format == nil || *devstral.Details.Format != "gguf" {
					t.Error("Expected format to be 'gguf'")
				}

				if devstral.Details.ParentModel == nil || *devstral.Details.ParentModel != "codestral" {
					t.Error("Expected parent_model to be 'codestral'")
				}

				if devstral.Details.Digest == nil || *devstral.Details.Digest != "c4b2fa0c33d75457e5f1c8507c906a79e73285768686db13b9cbac0c7ee3a854" {
					t.Error("Expected digest to be parsed correctly")
				}

				if devstral.Details.ModifiedAt == nil {
					t.Error("Expected modified_at to be parsed")
				}

				if len(devstral.Details.Families) != 1 || devstral.Details.Families[0] != "llama" {
					t.Error("Expected families array to contain 'llama'")
				}
			},
		},
		{
			name:               "LM Studio Platform Rich Metadata",
			platformType:       domain.ProfileLmStudio,
			expectedModelCount: 3,
			response: `{
				"object": "list",
				"data": [
					{
						"id": "meta-llama-3.3-8b-instruct",
						"object": "model",
						"type": "llm",
						"publisher": "lmstudio-community",
						"arch": "llama",
						"compatibility_type": "gguf",
						"quantization": "Q4_K_M",
						"state": "not-loaded",
						"max_context_length": 131072
					},
					{
						"id": "qwen3-vl-7b-instruct",
						"object": "model",
						"type": "vlm",
						"publisher": "mlx-community",
						"arch": "qwen3_vl",
						"compatibility_type": "mlx",
						"quantization": "4bit",
						"state": "loaded",
						"max_context_length": 32768
					},
					{
						"id": "text-embedding-nomic-embed-text-v1.5",
						"object": "model",
						"type": "embeddings",
						"publisher": "nomic-ai",
						"arch": "nomic-bert",
						"compatibility_type": "gguf",
						"quantization": "fp16",
						"state": "not-loaded",
						"max_context_length": 8192
					}
				]
			}`,
			validateMetadata: func(t *testing.T, models []*domain.ModelInfo) {
				var llama *domain.ModelInfo
				for _, model := range models {
					if model.Name == "meta-llama-3.3-8b-instruct" {
						llama = model
						break
					}
				}

				if llama == nil {
					t.Fatal("Expected to find meta-llama-3.3-8b-instruct model")
				}

				if llama.Details == nil {
					t.Fatal("Expected llama model to have details")
				}

				if llama.Details.Family == nil || *llama.Details.Family != "llama" {
					t.Error("Expected family to be mapped from arch field")
				}

				if llama.Details.QuantizationLevel == nil || *llama.Details.QuantizationLevel != "Q4_K_M" {
					t.Error("Expected quantization_level to be mapped from quantization field")
				}

				if llama.Details.Format == nil || *llama.Details.Format != "gguf" {
					t.Error("Expected format to be mapped from compatibility_type field")
				}

				if llama.Details.ParentModel == nil || *llama.Details.ParentModel != "lmstudio-community" {
					t.Error("Expected parent_model to be mapped from publisher field")
				}

				if llama.Details.State == nil || *llama.Details.State != "not-loaded" {
					t.Error("Expected state to be 'not-loaded'")
				}

				if llama.Details.MaxContextLength == nil || *llama.Details.MaxContextLength != 131072 {
					t.Error("Expected max_context_length to be 131072")
				}

				if llama.Details.Type == nil || *llama.Details.Type != "llm" {
					t.Error("Expected type to be 'llm'")
				}

				// Test VLM model
				var qwen *domain.ModelInfo
				for _, model := range models {
					if model.Name == "qwen3-vl-7b-instruct" {
						qwen = model
						break
					}
				}

				if qwen == nil {
					t.Fatal("Expected to find qwen3-vl-7b-instruct model")
				}

				if qwen.Details == nil {
					t.Fatal("Expected qwen model to have details")
				}

				if qwen.Details.Family == nil || *qwen.Details.Family != "qwen3_vl" {
					t.Error("Expected qwen family to be 'qwen3_vl'")
				}

				if qwen.Details.State == nil || *qwen.Details.State != "loaded" {
					t.Error("Expected qwen state to be 'loaded'")
				}

