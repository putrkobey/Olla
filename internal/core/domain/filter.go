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
