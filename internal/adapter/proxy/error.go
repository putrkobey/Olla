package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"
)

var (
	// ErrNoHealthyEndpoints is returned when no healthy endpoints are available
	ErrNoHealthyEndpoints = errors.New("no healthy endpoints available")
)

// MakeUserFriendlyError is the exported version for external packages
func MakeUserFriendlyError(err error, duration time.Duration, errorContext string, responseTimeout time.Duration) error {
	return makeUserFriendlyError(err, duration, errorContext, responseTimeout)
}

