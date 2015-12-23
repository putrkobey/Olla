// Package constants defines auth scheme and header constants used across
// the outbound request pipeline.
package constants

// Auth type identifiers. These are the valid values for endpoint auth.type in config.
const (
	AuthTypeBearer = "bearer"
	AuthTypeAPIKey = "api_key"
	AuthTypeBasic  = "basic"
)

// HTTP header names used when injecting auth onto outbound requests.
const (
	// AuthHeaderAuthorization is the standard header for bearer and basic auth.
	AuthHeaderAuthorization = "Authorization"

	// AuthDefaultAPIKeyHeader is the fallback header name when an api_key auth
	// block omits the optional header field.
