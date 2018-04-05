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

