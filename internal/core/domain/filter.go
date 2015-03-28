package domain

import (
	"errors"
	"fmt"
	"strings"
)

// FilterConfig represents the configuration for filtering items
// using include and exclude patterns with glob support
type FilterConfig struct {
	// Include patterns - items matching any of these patterns are included
	// If empty or contains "*", all items are initially included
	Include []string `yaml:"include,omitempty" json:"include,omitempty"`

	// Exclude patterns - items matching any of these patterns are excluded
	// Exclude takes precedence over Include
	Exclude []string `yaml:"exclude,omitempty" json:"exclude,omitempty"`
}

// FilterResult represents the result of applying a filter
type FilterResult struct {
	// Items that passed the filter
	Accepted []interface{} `json:"accepted"`

	// Items that were filtered out
	Rejected []interface{} `json:"rejected"`

	// Statistics about the filtering operation
	Stats FilterStats `json:"stats"`
}

// FilterStats provides statistics about a filtering operation
type FilterStats struct {
	TotalItems    int `json:"total_items"`
	AcceptedCount int `json:"accepted_count"`
	RejectedCount int `json:"rejected_count"`
}

// IsEmpty returns true if no filter patterns are configured
func (fc *FilterConfig) IsEmpty() bool {
	return (fc.Include == nil || len(fc.Include) == 0) &&
		(fc.Exclude == nil || len(fc.Exclude) == 0)
}

// HasIncludeAll returns true if the include patterns would include everything
func (fc *FilterConfig) HasIncludeAll() bool {
	if fc.Include == nil || len(fc.Include) == 0 {
		return true
	}

	for _, pattern := range fc.Include {
		if pattern == "*" {
			return true
		}
	}

	return false
}

// Validate checks if the filter configuration is valid
func (fc *FilterConfig) Validate() error {
	// Check for invalid patterns
	for _, pattern := range fc.Include {
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("invalid include pattern '%s': %w", pattern, err)
		}
	}

	for _, pattern := range fc.Exclude {
		if err := validatePattern(pattern); err != nil {
			return fmt.Errorf("invalid exclude pattern '%s': %w", pattern, err)
		}
	}

	return nil
}

// validatePattern checks if a glob pattern is valid
func validatePattern(pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return errors.New("empty or whitespace-only pattern")
	}

	// Check for invalid characters or patterns
	// We support simple glob patterns with * wildcard
	if strings.Contains(pattern, "**") {
		return errors.New("double asterisk not supported")
	}

	// Count asterisks - we support at most one at start and/or end
	asteriskCount := strings.Count(pattern, "*")
	if asteriskCount > 2 {
		return errors.New("too many wildcards")
	}

	if asteriskCount == 1 {
		// Single wildcard should be at start or end
		if !strings.HasPrefix(pattern, "*") && !strings.HasSuffix(pattern, "*") {
			return errors.New("wildcards only supported at start and/or end")
		}
	} else if asteriskCount == 2 {
		// Two wildcards should only be at start and end for *text* pattern
		if !strings.HasPrefix(pattern, "*") || !strings.HasSuffix(pattern, "*") {
			return errors.New("wildcards only supported at start and/or end")
		}
	}

	return nil
}

// Clone creates a deep copy of the FilterConfig
func (fc *FilterConfig) Clone() *FilterConfig {
	if fc == nil {
		return nil
	}

	clone := &FilterConfig{}

	if fc.Include != nil {
		clone.Include = make([]string, len(fc.Include))
		copy(clone.Include, fc.Include)
	}

