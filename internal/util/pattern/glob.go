package pattern

import "strings"

// MatchesGlob checks if a string matches a glob pattern with * wildcard support
// This is the centralised pattern matching logic used throughout Olla
func MatchesGlob(s, pattern string) bool {
