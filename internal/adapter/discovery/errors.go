package discovery

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// DiscoveryError wraps discovery operation errors with context
type DiscoveryError struct {
	Err         error
	EndpointURL string
	ProfileType string
	Operation   string
	StatusCode  int
	Latency     time.Duration
}

func (e *DiscoveryError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("discovery %s failed for %s (profile: %s, status: %d, latency: %v): %v",
			e.Operation, e.EndpointURL, e.ProfileType, e.StatusCode, e.Latency, e.Err)
	}
	return fmt.Sprintf("discovery %s failed for %s (profile: %s, latency: %v): %v",
		e.Operation, e.EndpointURL, e.ProfileType, e.Latency, e.Err)
}

func (e *DiscoveryError) Unwrap() error {
	return e.Err
}

func NewDiscoveryError(endpointURL, profileType, operation string, statusCode int, latency time.Duration, err error) *DiscoveryError {
	return &DiscoveryError{
		EndpointURL: endpointURL,
		ProfileType: profileType,
		Operation:   operation,
		StatusCode:  statusCode,
		Latency:     latency,
		Err:         err,
	}
}

// ErrResponseTooLarge marks a model listing that exceeded MaxResponseSize. It is
// matched with errors.Is so retry logic need not parse the message.
var ErrResponseTooLarge = errors.New("model response too large")

// ProfileNotFoundError indicates no suitable profile was found for an endpoint
type ProfileNotFoundError struct {
	ProfileType string
}

func (e *ProfileNotFoundError) Error() string {
	return fmt.Sprintf("profile not found: %s", e.ProfileType)
}

// ParseError indicates response parsing failed
type ParseError struct {
	Err    error
	Format string
	Data   []byte
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse %s response: %v", e.Format, e.Err)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// NetworkError indicates a network-level failure
type NetworkError struct {
	Err error
	URL string
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error for %s: %v", e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// GetUserFriendlyMessage gives a concise, user-friendly error message from discovery errors
func GetUserFriendlyMessage(err error) string {
	var parseErr *ParseError
	if errors.As(err, &parseErr) {
		return "invalid response format"
	}

	var discErr *DiscoveryError
	if errors.As(err, &discErr) {
		if discErr.StatusCode > 0 && discErr.StatusCode != 200 {
			switch {
			case discErr.StatusCode >= 400 && discErr.StatusCode < 500:
				return fmt.Sprintf("endpoint configuration issue (HTTP %d)", discErr.StatusCode)
			case discErr.StatusCode >= 500:
				return fmt.Sprintf("endpoint server error (HTTP %d)", discErr.StatusCode)
			default:
				return fmt.Sprintf("HTTP error (%d)", discErr.StatusCode)
			}
		}

		if discErr.Operation == "read_response" {
			return "failed to read model response"
		}

		// common network issues
		if discErr.Err != nil {
			errStr := discErr.Err.Error()
			switch {
