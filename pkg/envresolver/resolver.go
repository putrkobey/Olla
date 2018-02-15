// Package envresolver expands ${VAR} and ${VAR:-default} placeholders in
// configuration strings using environment variable lookups. It intentionally
// does not support the bare $VAR form: config files often contain literal
// dollar signs (shell scripts, cost strings, regex), and requiring braces
// eliminates ambiguity without meaningful ergonomic cost.
package envresolver

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// tokenPattern matches ${VAR} and ${VAR:-default}. No nesting, no bare $VAR.
var tokenPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Expand replaces every ${VAR} and ${VAR:-default} placeholder in s with its
// resolved value. An unset variable with no default resolves to the empty
// string. Expand never returns an error; use ExpandStrict when a missing
// variable must be fatal.
func Expand(s string) string {
