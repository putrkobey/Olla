package pattern

import "strings"

// MatchesGlob checks if a string matches a glob pattern with * wildcard support
// This is the centralised pattern matching logic used throughout Olla
func MatchesGlob(s, pattern string) bool {
	// glob matching for * wildcard
	pattern = strings.ToLower(pattern)
	s = strings.ToLower(s)

	switch {
	case pattern == "*":
		return true
	case strings.Contains(pattern, "*"):
