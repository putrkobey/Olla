package domain

import (
	"fmt"
	"testing"
)

func TestNewRequestProfile(t *testing.T) {
	path := "/v1/chat/completions"
	profile := NewRequestProfile(path)

	if profile == nil {
		t.Fatal("NewRequestProfile() should not return nil")
	}

	if profile.Path != path {
		t.Errorf("NewRequestProfile() path = %v, want %v", profile.Path, path)
	}

	if profile.SupportedBy == nil {
		t.Error("NewRequestProfile() should initialize SupportedBy slice")
	}

	if len(profile.SupportedBy) != 0 {
		t.Errorf("NewRequestProfile() should start with empty SupportedBy, got %v", profile.SupportedBy)
	}

	if profile.ModelName != "" {
		t.Errorf("NewRequestProfile() should start with empty ModelName, got %v", profile.ModelName)
	}

	if profile.InspectionMeta == nil {
		t.Error("NewRequestProfile() should initialize InspectionMeta map")
	}

	// test that the xsync map is functional
	profile.InspectionMeta.Store("test", "value")
	value, exists := profile.InspectionMeta.Load("test")
	if !exists || value != "value" {
		t.Error("NewRequestProfile() InspectionMeta should be functional")
	}
}

func TestRequestProfile_AddSupportedProfile(t *testing.T) {
	profile := NewRequestProfile("/test")

	// fist profile
	profile.AddSupportedProfile(ProfileOllama)
	if len(profile.SupportedBy) != 1 {
		t.Errorf("AddSupportedProfile() should add profile, got length %d", len(profile.SupportedBy))
	}
	if profile.SupportedBy[0] != ProfileOllama {
		t.Errorf("AddSupportedProfile() should add correct profile, got %v", profile.SupportedBy[0])
	}

	// another
	profile.AddSupportedProfile(ProfileLmStudio)
	if len(profile.SupportedBy) != 2 {
		t.Errorf("AddSupportedProfile() should add second profile, got length %d", len(profile.SupportedBy))
	}
	if !contains(profile.SupportedBy, ProfileLmStudio) {
		t.Errorf("AddSupportedProfile() should include LmStudio, got %v", profile.SupportedBy)
	}

	// whats better than one Ollama? Two!
	profile.AddSupportedProfile(ProfileOllama)
	if len(profile.SupportedBy) != 2 {
		t.Errorf("AddSupportedProfile() should not duplicate profiles, got length %d", len(profile.SupportedBy))
	}

	// empty one that should be ignored
	originalLength := len(profile.SupportedBy)
	profile.AddSupportedProfile("")
	if len(profile.SupportedBy) != originalLength {
		t.Error("AddSupportedProfile() should ignore empty profile names")
	}
}

func TestRequestProfile_IsCompatibleWith_AutoType(t *testing.T) {
	profile := NewRequestProfile("/test")

	// auto should be compatible
	if !profile.IsCompatibleWith(ProfileAuto) {
		t.Error("IsCompatibleWith() should always accept auto type")
	}

	// and with existing profiles
	profile.AddSupportedProfile(ProfileOllama)
	if !profile.IsCompatibleWith(ProfileAuto) {
		t.Error("IsCompatibleWith() should always accept auto type even with specific profiles")
	}
}

func TestRequestProfile_IsCompatibleWith_EmptySupported(t *testing.T) {
	profile := NewRequestProfile("/test")

	testTypes := []string{ProfileOllama, ProfileLmStudio, ProfileOpenAICompatible, "custom-type"}

	for _, profileType := range testTypes {
		if !profile.IsCompatibleWith(profileType) {
			t.Errorf("IsCompatibleWith() with empty supported list should accept %v", profileType)
		}
	}
}

func TestRequestProfile_IsCompatibleWith_ExactMatch(t *testing.T) {
	profile := NewRequestProfile("/test")
	profile.AddSupportedProfile(ProfileOllama)
	profile.AddSupportedProfile(ProfileLmStudio)

	if !profile.IsCompatibleWith(ProfileOllama) {
		t.Error("IsCompatibleWith() should match exact profile Ollama")
	}

	if !profile.IsCompatibleWith(ProfileLmStudio) {
		t.Error("IsCompatibleWith() should match exact profile LmStudio")
	}

	if profile.IsCompatibleWith("unsupported-profile") {
		t.Error("IsCompatibleWith() should not match unsupported profile")
	}
}

func TestRequestProfile_IsCompatibleWith_OpenAICompatibility(t *testing.T) {
	profile := NewRequestProfile("/test")
	profile.AddSupportedProfile(ProfileOpenAICompatible)

	// openai should be compatible with Ollama and LMStudio
	if !profile.IsCompatibleWith(ProfileOllama) {
		t.Error("IsCompatibleWith() OpenAI compatible should work with Ollama")
	}

	if !profile.IsCompatibleWith(ProfileLmStudio) {
		t.Error("IsCompatibleWith() OpenAI compatible should work with LMStudio")
	}

	if !profile.IsCompatibleWith(ProfileOpenAICompatible) {
		t.Error("IsCompatibleWith() should match OpenAI compatible exactly")
	}

	// Endpoints configured type:"openai" must also pass — this was the root cause of issue #148.
	// The GET /olla/openai/v1/models handler builds a profile with SupportedBy=[openai-compatible]
	// then filters endpoints; without this alias, type:"openai" endpoints were silently dropped.
	if !profile.IsCompatibleWith(ProfileOpenAI) {
		t.Error("IsCompatibleWith() OpenAI compatible should accept endpoints typed as 'openai'")
	}

	// but not with unsupported types
	if profile.IsCompatibleWith("unknown-type") {
		t.Error("IsCompatibleWith() OpenAI compatible should not work with unknown types")
	}
}

func TestRequestProfile_IsCompatibleWith_SpecificPlatforms(t *testing.T) {
