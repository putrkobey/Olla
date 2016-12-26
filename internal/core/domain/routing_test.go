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
