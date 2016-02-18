package profile

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/putrkobey/Olla/internal/core/domain"
)

type ProfileFactory interface {
	GetProfile(profileType string) (domain.InferenceProfile, error)
	GetAvailableProfiles() []string
	ReloadProfiles() error
	ValidateProfileType(platformType string) bool
	NormalizeProviderName(providerName string) string
}

type Factory struct {
	loader       *ProfileLoader
	prefixLookup map[string]string // maps URL prefix to profile name
	mu           sync.RWMutex
}

// NewFactory loads provider profiles from the specified directory.
// The explicit path requirement enables better testing and deployment flexibility.
func NewFactory(profilesDir string) (*Factory, error) {
	loader := NewProfileLoader(profilesDir)

	if err := loader.LoadProfiles(); err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	factory := &Factory{
		loader:       loader,
		prefixLookup: make(map[string]string),
	}

	// Pre-compute prefix mappings for O(1) provider resolution
