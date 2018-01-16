// Package constants defines auth scheme and header constants used across
// the outbound request pipeline.
package constants

// Auth type identifiers. These are the valid values for endpoint auth.type in config.
const (
	AuthTypeBearer = "bearer"
	AuthTypeAPIKey = "api_key"
	AuthTypeBasic  = "basic"
