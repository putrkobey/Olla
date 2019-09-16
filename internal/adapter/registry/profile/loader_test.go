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

