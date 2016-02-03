package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadProfiles_MalformedYAMLSurfacesWarning is the regression test for
// the same silent-drop bug class as issue #204: a profile file that exists
// but fails to parse used to be reported with a bare fmt.Printf and then
// vanish - LoadProfiles() itself returned no error, so NewFactory never knew
// anything went wrong. It must now show up in LoadWarnings(), and must not
// stop the rest of the directory (valid custom profiles, and the built-ins)
// from loading.
func TestLoadProfiles_MalformedYAMLSurfacesWarning(t *testing.T) {
	dir := t.TempDir()

	// invalid YAML: unbalanced flow mapping
	broken := "name: broken-profile\nrouting: {prefixes: [broken\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.yaml"), []byte(broken), 0644))

	valid := "name: custom-valid\nversion: \"1.0\"\nrouting:\n  prefixes:\n    - custom-valid\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "valid.yaml"), []byte(valid), 0644))

	loader := NewProfileLoader(dir)
	require.NoError(t, loader.LoadProfiles(), "LoadProfiles must not fail outright for one bad file")

	warnings := loader.LoadWarnings()
	require.Len(t, warnings, 1, "expected exactly one load warning")
	assert.Contains(t, warnings[0], "broken.yaml", "warning should name the broken file")

	// the valid custom profile must still have loaded
	_, ok := loader.GetProfile("custom-valid")
	assert.True(t, ok, "valid profile in the same directory should still load")

	// built-ins must still be present - a bad custom file must not wipe them out
	_, ok = loader.GetProfile("ollama")
	assert.True(t, ok, "built-in ollama profile should still be present")
}
