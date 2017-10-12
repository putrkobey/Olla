package profile

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/adapter/filter"
	"github.com/putrkobey/Olla/internal/core/constants"
	"github.com/putrkobey/Olla/internal/core/domain"
	"github.com/putrkobey/Olla/internal/core/ports"
	"gopkg.in/yaml.v3"
)

type ProfileLoader struct {
	filter        ports.Filter
	profiles      map[string]domain.InferenceProfile
	profileFilter *domain.FilterConfig
	profilesDir   string
	loadWarnings  []string
	mu            sync.RWMutex
}

func NewProfileLoader(profilesDir string) *ProfileLoader {
	return &ProfileLoader{
		profilesDir: profilesDir,
		profiles:    make(map[string]domain.InferenceProfile),
		filter:      filter.NewGlobFilter(),
	}
}

// NewProfileLoaderWithFilter creates a new ProfileLoader with a custom filter
func NewProfileLoaderWithFilter(profilesDir string, profileFilter *domain.FilterConfig, customFilter ports.Filter) *ProfileLoader {
	filterToUse := customFilter
	if filterToUse == nil {
		filterToUse = filter.NewGlobFilter()
	}

	return &ProfileLoader{
		profilesDir:   profilesDir,
		profiles:      make(map[string]domain.InferenceProfile),
		profileFilter: profileFilter,
		filter:        filterToUse,
	}
}

const DefaultModelKey = "model"
const DefaultModelsUri = "/v1/models"
const defaultNameFormat = "{{.Name}}"

func (l *ProfileLoader) LoadProfiles() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// reset per-call so a reload doesn't accumulate warnings from a
	// previously-broken file that's since been fixed or removed
	l.loadWarnings = nil

	allProfiles := make(map[string]domain.InferenceProfile)

	// built-ins ensure it works out of the box, even without config files
	l.loadBuiltInProfilesInto(allProfiles)

	if _, err := os.Stat(l.profilesDir); os.IsNotExist(err) {
		// no config dir is fine - built-ins cover the common cases
		// Apply filtering before returning
		return l.applyProfileFilter(allProfiles)
	}

	// yaml files in config dir override built-ins
	err := filepath.WalkDir(l.profilesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		profile, err := l.loadProfile(path)
		if err != nil {
			// don't fail everything because of one bad yaml file - but don't
			// go silent either (issue #204's bug class): record it so
			// --validate-config and any caller with a real logger can
			// surface it, and warn now via slog so it shows up in ordinary
			// startup logs too.
			warning := fmt.Sprintf("%s: %v", path, err)
			l.loadWarnings = append(l.loadWarnings, warning)
			slog.Warn("profile failed to load, skipping", "path", path, "error", err)
			return nil
		}

		allProfiles[profile.GetName()] = profile
		return nil
	})

	if err != nil {
		return err
	}

	// Apply filtering to all loaded profiles
	return l.applyProfileFilter(allProfiles)
}

// LoadWarnings returns any profile files that were found but failed to load
// during the most recent LoadProfiles call - e.g. malformed YAML or a
// missing required field. Empty when every discovered profile loaded
// cleanly. Exposed so callers like --validate-config can surface these
// without depending on log capture.
func (l *ProfileLoader) LoadWarnings() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return append([]string(nil), l.loadWarnings...)
}

// applyProfileFilter applies the configured filter to the profiles
func (l *ProfileLoader) applyProfileFilter(allProfiles map[string]domain.InferenceProfile) error {
	// If no filter is configured, use all profiles
	if l.profileFilter == nil || l.profileFilter.IsEmpty() {
		l.profiles = allProfiles
		return nil
	}

	// Apply the filter
	ctx := context.Background()
	filteredProfiles, err := l.filter.ApplyToMap(ctx, l.profileFilter, convertProfilesToMap(allProfiles))
	if err != nil {
		return fmt.Errorf("failed to apply profile filter: %w", err)
	}

	// Convert back to typed map
	l.profiles = make(map[string]domain.InferenceProfile)
	for name, profile := range filteredProfiles {
		if p, ok := profile.(domain.InferenceProfile); ok {
			l.profiles[name] = p
		}
	}

	return nil
}

// convertProfilesToMap converts typed profile map to generic map for filtering
func convertProfilesToMap(profiles map[string]domain.InferenceProfile) map[string]interface{} {
	result := make(map[string]interface{}, len(profiles))
	for name, profile := range profiles {
		result[name] = profile
	}
	return result
}

func (l *ProfileLoader) loadProfile(path string) (domain.InferenceProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	var config domain.ProfileConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse profile YAML: %w", err)
	}

