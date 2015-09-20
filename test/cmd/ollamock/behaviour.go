package main

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
)

// Mode describes how ollamock should behave when processing requests.
type Mode string

const (
	// ModeOK is normal operation - all requests succeed.
	ModeOK Mode = "ok"
	// ModeError causes all requests to return a fixed error status.
	ModeError Mode = "error"
	// ModeFlaky returns errors at a configurable rate so tests can exercise
	// retry and circuit-breaker logic.
	ModeFlaky Mode = "flaky"
	// ModeHang blocks requests indefinitely (until context cancellation) to
	// simulate a hung backend.
	ModeHang Mode = "hang"
	// ModeSlow adds a fixed latency to every response.
	ModeSlow Mode = "slow"
)

// validModes is the closed set of accepted mode strings.
var validModes = map[Mode]bool{
	ModeOK:    true,
	ModeError: true,
	ModeFlaky: true,
	ModeHang:  true,
	ModeSlow:  true,
}

// Behaviour captures the current fault-injection configuration.
// Field order is largest-to-smallest for alignment.
type Behaviour struct {
	Mode          Mode    `json:"mode"`
	ErrorRate     float64 `json:"error_rate"`
	LatencyMS     int     `json:"latency_ms"`
	ErrorStatus   int     `json:"error_status"`
	FailHealth    bool    `json:"fail_health"`
	DropMidStream bool    `json:"drop_mid_stream"`
	MalformedJSON bool    `json:"malformed_json"`
}

func defaultBehaviour() Behaviour {
	return Behaviour{
		Mode:        ModeOK,
		ErrorStatus: 500,
		ErrorRate:   0.5,
	}
}
