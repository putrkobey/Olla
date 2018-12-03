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
