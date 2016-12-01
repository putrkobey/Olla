package profile

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// Helper function to get profiles from factory for tests
func getTestProfile(t *testing.T, profileType string) domain.PlatformProfile {
	t.Helper()
	factory := testFactory(t)
	profile, err := factory.GetProfile(profileType)
	if err != nil {
		t.Fatalf("Failed to get %s profile: %v", profileType, err)
	}
	return profile
}

func TestOllamaProfile(t *testing.T) {
	factory := testFactory(t)
	profile, err := factory.GetProfile(domain.ProfileOllama)
	if err != nil {
		t.Fatalf("Failed to get ollama profile: %v", err)
	}

	if profile.GetName() != domain.ProfileOllama {
		t.Errorf("Expected name %q, got %q", domain.ProfileOllama, profile.GetName())
	}

	url := profile.GetModelDiscoveryURL("http://localhost:11434")
	expected := "http://localhost:11434/api/tags"
	if url != expected {
		t.Errorf("Expected %q, got %q", expected, url)
	}

	url = profile.GetModelDiscoveryURL("http://localhost:11434/")
	if url != expected {
		t.Errorf("Expected %q, got %q", expected, url)
	}

	if !profile.IsOpenAPICompatible() {
		t.Error("ProfileOllama should be OpenAPI compatible")
	}

	rules := profile.GetRequestParsingRules()
	if rules.ModelFieldName != "model" {
		t.Errorf("Expected model field 'model', got %q", rules.ModelFieldName)
	}
	if !rules.SupportsStreaming {
		t.Error("ProfileOllama should support streaming")
	}

	format := profile.GetModelResponseFormat()
	if format.ResponseType != "object" {
		t.Errorf("Expected response type 'object', got %q", format.ResponseType)
	}
	if format.ModelsFieldPath != "models" {
		t.Errorf("Expected models field path 'models', got %q", format.ModelsFieldPath)
	}
}

func TestLMStudioProfile(t *testing.T) {
	factory := testFactory(t)
	profile, err := factory.GetProfile(domain.ProfileLmStudio)
	if err != nil {
		t.Fatalf("Failed to get lmstudio profile: %v", err)
	}

	if profile.GetName() != domain.ProfileLmStudio {
		t.Errorf("Expected name %q, got %q", domain.ProfileLmStudio, profile.GetName())
	}

	baseUrl := "http://localhost:1234"
	url := profile.GetModelDiscoveryURL(baseUrl)
	expected := baseUrl + "/api/v0/models"
	if url != expected {
		t.Errorf("Expected %q, got %q", expected, url)
	}

	if !profile.IsOpenAPICompatible() {
		t.Error("LM Studio should be OpenAPI compatible")
	}

	rules := profile.GetRequestParsingRules()
	if rules.ModelFieldName != "model" {
		t.Errorf("Expected model field 'model', got %q", rules.ModelFieldName)
	}
	if rules.GeneratePath != "" {
		t.Error("LM Studio should not have generate path")
	}

	format := profile.GetModelResponseFormat()
	if format.ModelsFieldPath != "data" {
		t.Errorf("Expected models field path 'data', got %q", format.ModelsFieldPath)
	}
}

func TestOpenAICompatibleProfile(t *testing.T) {
	factory := testFactory(t)
	profile, err := factory.GetProfile(domain.ProfileOpenAICompatible)
	if err != nil {
		t.Fatalf("Failed to get openai compatible profile: %v", err)
	}

	if profile.GetName() != domain.ProfileOpenAICompatible {
		t.Errorf("Expected name %q, got %q", domain.ProfileOpenAICompatible, profile.GetName())
	}

	url := profile.GetModelDiscoveryURL("http://localhost:8080")
	expected := "http://localhost:8080/v1/models"
	if url != expected {
		t.Errorf("Expected %q, got %q", expected, url)
	}

	if !profile.IsOpenAPICompatible() {
		t.Error("OpenAI compatible profile should be OpenAPI compatible")
	}

	rules := profile.GetRequestParsingRules()
	if !rules.SupportsStreaming {
		t.Error("OpenAI compatible should support streaming")
	}

	format := profile.GetModelResponseFormat()
	if format.ResponseType != "object" {
		t.Errorf("Expected response type 'object', got %q", format.ResponseType)
	}

	// OpenAI compatible should have minimal metadata support
	if format.ModelsFieldPath != "data" {
		t.Errorf("Expected models field path 'data', got %q", format.ModelsFieldPath)
	}
}

func TestProfileVersioning(t *testing.T) {
	profiles := []domain.PlatformProfile{
		getTestProfile(t, domain.ProfileOllama),
		getTestProfile(t, domain.ProfileLmStudio),
		getTestProfile(t, domain.ProfileOpenAICompatible),
	}

	for _, profile := range profiles {
		version := profile.GetVersion()
		if version == "" {
			t.Errorf("Profile %s should have a version", profile.GetName())
		}
	}
}

func TestDetectionHints(t *testing.T) {
	profile := getTestProfile(t, domain.ProfileOllama)
	hints := profile.GetDetectionHints()

	if len(hints.PathIndicators) == 0 {
		t.Error("ProfileOllama should have path indicators for detection")
	}

	found := false
	for _, path := range hints.PathIndicators {
		if path == "/api/tags" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ProfileOllama should have /api/tags as path indicator")
	}
}

// Migrated and enhanced tests from parser_test.go
func TestParseModelsResponse(t *testing.T) {
	tests := []struct {
		name           string
		profile        domain.PlatformProfile
		responseBody   []byte
		expectedModels int
		expectedError  bool
		expectedNames  []string
		validateModel  func(*testing.T, *domain.ModelInfo, string)
	}{
		{
			name:    "Ollama response format with rich metadata",
			profile: getTestProfile(t, domain.ProfileOllama),
			responseBody: []byte(`{
				"models": [
					{
						"name": "devstral:latest",
						"size": 14333927918,
						"digest": "c4b2fa0c33d75457e5f1c8507c906a79e73285768686db13b9cbac0c7ee3a854",
						"modified_at": "2025-05-30T14:24:44.5116551+10:00",
						"details": {
							"parameter_size": "23.6B",
							"quantization_level": "Q4_K_M",
							"family": "llama",
							"families": ["llama"],
							"format": "gguf",
							"parent_model": ""
						}
					},
					{
						"name": "codegemma:9b",
						"size": 5011852809,
						"description": "Code generation model"
					}
				]
			}`),
			expectedModels: 2,
			expectedError:  false,
			expectedNames:  []string{"devstral:latest", "codegemma:9b"},
			validateModel: func(t *testing.T, model *domain.ModelInfo, name string) {
				if name == "devstral:latest" {
					if model.Size != 14333927918 {
						t.Errorf("Expected size 14333927918, got %d", model.Size)
					}
					if model.Details == nil {
						t.Fatal("Expected details to be parsed")
					}
					if model.Details.ParameterSize == nil || *model.Details.ParameterSize != "23.6B" {
						t.Error("Expected parameter_size to be '23.6B'")
					}
					if model.Details.QuantizationLevel == nil || *model.Details.QuantizationLevel != "Q4_K_M" {
						t.Error("Expected quantization_level to be 'Q4_K_M'")
					}
					if model.Details.Family == nil || *model.Details.Family != "llama" {
						t.Error("Expected family to be 'llama'")
					}
					if model.Details.Digest == nil || *model.Details.Digest != "c4b2fa0c33d75457e5f1c8507c906a79e73285768686db13b9cbac0c7ee3a854" {
						t.Error("Expected digest to be parsed correctly")
					}
					if len(model.Details.Families) != 1 || model.Details.Families[0] != "llama" {
						t.Error("Expected families array to contain 'llama'")
					}
				}
			},
		},
		{
			name:    "LM Studio response format with rich metadata",
			profile: getTestProfile(t, domain.ProfileLmStudio),
			responseBody: []byte(`{
				"object": "list",
				"data": [
					{
						"id": "meta-llama-3.1-8b-instruct",
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
						"id": "microsoft/DialoGPT-medium",
						"object": "model",
						"created": 1686935002,
						"owned_by": "microsoft"
					}
				]
			}`),
			expectedModels: 2,
			expectedError:  false,
			expectedNames:  []string{"meta-llama-3.1-8b-instruct", "microsoft/DialoGPT-medium"},
			validateModel: func(t *testing.T, model *domain.ModelInfo, name string) {
				if name == "meta-llama-3.1-8b-instruct" {
					if model.Type != "model" {
						t.Errorf("Expected type 'model', got %s", model.Type)
					}
					if model.Details == nil {
						t.Fatal("Expected details to be parsed for LM Studio")
					}
					if model.Details.Family == nil || *model.Details.Family != "llama" {
						t.Error("Expected family to be 'llama'")
					}
					if model.Details.QuantizationLevel == nil || *model.Details.QuantizationLevel != "Q4_K_M" {
						t.Error("Expected quantization_level to be 'Q4_K_M'")
					}
					if model.Details.Format == nil || *model.Details.Format != "gguf" {
						t.Error("Expected format to be 'gguf'")
					}
					if *model.Details.MaxContextLength != 131072 {
						t.Errorf("Expected MaxContextLength to be 131072 but got %d", *model.Details.MaxContextLength)
					}
				}
			},
		},
		{
			name:    "OpenAI compatible response format",
			profile: getTestProfile(t, domain.ProfileOpenAICompatible),
			responseBody: []byte(`{
				"object": "list",
				"data": [
					{
						"id": "gpt-3.5-turbo",
						"object": "model",
						"created": 1677610602,
						"owned_by": "openai"
					},
					{
						"id": "text-davinci-003",
						"object": "model",
						"created": 1669599635,
						"owned_by": "openai-internal"
					}
				]
			}`),
			expectedModels: 2,
			expectedError:  false,
			expectedNames:  []string{"gpt-3.5-turbo", "text-davinci-003"},
			validateModel: func(t *testing.T, model *domain.ModelInfo, name string) {
				if model.Type != "model" {
					t.Errorf("Expected type 'model', got %s", model.Type)
				}
				// OpenAI compatible should have minimal metadata
				if name == "gpt-3.5-turbo" && model.Details != nil && model.Details.ModifiedAt != nil {
					// Should have parsed created timestamp
					expectedTime := time.Unix(1677610602, 0)
					if !model.Details.ModifiedAt.Equal(expectedTime) {
						t.Errorf("Expected modified_at to be %v, got %v", expectedTime, *model.Details.ModifiedAt)
