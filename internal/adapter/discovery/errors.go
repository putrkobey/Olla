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

