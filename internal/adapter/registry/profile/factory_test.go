package profile

import (
	"testing"

	"github.com/putrkobey/Olla/internal/core/domain"
)

// testFactory creates a factory for tests, using built-in profiles
func testFactory(t *testing.T) *Factory {
	t.Helper()
	factory, err := NewFactory("") // Empty dir uses built-in profiles
	if err != nil {
		t.Fatalf("Failed to create test factory: %v", err)
	}
	return factory
}

func TestNewFactory(t *testing.T) {
	factory := testFactory(t)

	profiles := factory.GetAvailableProfiles()
	expectedProfiles := []string{domain.ProfileLmStudio, domain.ProfileOllama, domain.ProfileLlamaCpp}

	// Built-in profiles: Ollama, LM Studio, llama.cpp (not OpenAI-compatible since it's fallback)
	if len(profiles) != len(expectedProfiles) {
		t.Errorf("Expected %d profiles, got %d (profiles: %v)", len(expectedProfiles), len(profiles), profiles)
	}

	for _, expected := range expectedProfiles {
		found := false
		for _, actual := range profiles {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected profile %q not found", expected)
		}
	}
}

func TestGetProfile(t *testing.T) {
	factory := testFactory(t)

	profile, err := factory.GetProfile(domain.ProfileOllama)
	if err != nil {
		t.Fatalf("Failed to get ollama profile: %v", err)
	}
	if profile.GetName() != domain.ProfileOllama {
		t.Errorf("Expected ollama profile, got %q", profile.GetName())
	}

	profile, err = factory.GetProfile("nonexistent")
	if err != nil {
		t.Fatalf("Should return OpenAI compatible profile for nonexistent type: %v", err)
	}
	if profile.GetName() != domain.ProfileOpenAICompatible {
		t.Errorf("Expected OpenAI compatible profile for nonexistent type, got %q", profile.GetName())
	}
}

func TestValidateProfileType(t *testing.T) {
	factory := testFactory(t)

	if !factory.ValidateProfileType(domain.ProfileOllama) {
		t.Error("ollama should be valid profile type")
	}
	if !factory.ValidateProfileType(domain.ProfileAuto) {
		t.Error("auto should be valid profile type")
	}
	if factory.ValidateProfileType("invalid") {
		t.Error("invalid should not be valid profile type")
	}
}

func TestValidateProfileType_WithRoutingPrefixes(t *testing.T) {
	// Create factory with default profiles
	factory := testFactory(t)

	tests := []struct {
