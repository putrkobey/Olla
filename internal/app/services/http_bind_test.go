package services

import (
	"context"
	"net"
	"testing"
)

// TestBindListener_FreePort verifies that bindListener returns a valid listener
// when no other process holds the port.
func TestBindListener_FreePort(t *testing.T) {
	t.Parallel()

	ln, err := bindListener(context.Background(), "127.0.0.1:0")
	if err != nil {
		t.Fatalf("expected bind to succeed on a free port: %v", err)
