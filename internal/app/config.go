package app

import (
	"time"
)

const (
	DefaultConnectionTimeout   = 30 * time.Second
	DefaultConnectionKeepAlive = 30 * time.Second
	DefaultResponseTimeout     = 600 * time.Second
	DefaultReadTimeout         = 300 * time.Second
	DefaultLoadBalancer        = "priority"
	DefaultStreamBufferSize    = 8 * 1024 // 8KB

	// DefaultReadHeaderTimeout guards the inbound server against Slowloris-style
