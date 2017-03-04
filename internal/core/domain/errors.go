package domain

import (
	"fmt"
	"time"
)

type EndpointError struct {
	Err       error
	Operation string
	URL       string
}

func (e *EndpointError) Error() string {
	return fmt.Sprintf("%s failed for endpoint %s: %v", e.Operation, e.URL, e.Err)
}

func (e *EndpointError) Unwrap() error {
	return e.Err
}

type HealthCheckError struct {
	Err                 error
	EndpointURL         string
	EndpointName        string
	CheckType           string
	StatusCode          int
	Latency             time.Duration
	ConsecutiveFailures int
}

func (e *HealthCheckError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("health check failed for %s (%s): HTTP %d after %v (failures: %d): %v",
			e.EndpointName, e.EndpointURL, e.StatusCode, e.Latency, e.ConsecutiveFailures, e.Err)
	}
	return fmt.Sprintf("health check failed for %s (%s): %v after %v (failures: %d)",
		e.EndpointName, e.EndpointURL, e.Err, e.Latency, e.ConsecutiveFailures)
}

func (e *HealthCheckError) Unwrap() error {
	return e.Err
}

type ProxyError struct {
	Err        error
	RequestID  string
	TargetURL  string
	Method     string
	Path       string
	StatusCode int
	Latency    time.Duration
