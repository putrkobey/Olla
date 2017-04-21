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

// behaviourState holds the mutable behaviour alongside a seeded RNG for flaky
// mode. Both are protected by the same mutex so reads and writes are always
// consistent - the rand source is not goroutine-safe on its own.
type behaviourState struct {
	rng  *rand.Rand
	b    Behaviour
	seed int64
	mu   sync.RWMutex
}

func newBehaviourState(seed int64) *behaviourState {
	return &behaviourState{
		b:    defaultBehaviour(),
		rng:  rand.New(rand.NewSource(seed)), //nolint:gosec // deterministic seed for reproducible test scenarios
		seed: seed,
	}
}

func (s *behaviourState) get() Behaviour {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.b
}

// merge applies a partial JSON patch into the current behaviour.
// Only fields present in the patch are updated; others retain their value.
func (s *behaviourState) merge(patch Behaviour, hasPatch patchFields) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if hasPatch.mode {
		if !validModes[patch.Mode] {
			return errUnknownMode
		}
		s.b.Mode = patch.Mode
	}
	if hasPatch.errorStatus {
		s.b.ErrorStatus = patch.ErrorStatus
	}
	if hasPatch.errorRate {
		s.b.ErrorRate = patch.ErrorRate
	}
	if hasPatch.latencyMS {
		s.b.LatencyMS = patch.LatencyMS
	}
	if hasPatch.failHealth {
		s.b.FailHealth = patch.FailHealth
	}
	if hasPatch.dropMidStream {
		s.b.DropMidStream = patch.DropMidStream
	}
	if hasPatch.malformedJSON {
		s.b.MalformedJSON = patch.MalformedJSON
	}
	return nil
}

func (s *behaviourState) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.b = defaultBehaviour()
	// Re-seed with the original seed so behaviour after reset is reproducible.
	s.rng = rand.New(rand.NewSource(s.seed)) //nolint:gosec
}

// float64 returns a random float64 in [0,1). Caller must not hold the mutex.
func (s *behaviourState) float64() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64()
}

// patchFields tracks which JSON fields were explicitly set in a PATCH body.
// This lets us distinguish "error_rate: 0" (explicit zero) from absent field.
type patchFields struct {
	mode          bool
	errorStatus   bool
	errorRate     bool
	latencyMS     bool
	failHealth    bool
	dropMidStream bool
	malformedJSON bool
}

// parsePatch decodes a partial behaviour JSON and records which fields were present.
func parsePatch(data []byte) (Behaviour, patchFields, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Behaviour{}, patchFields{}, err
	}

	var b Behaviour
	var pf patchFields

	if v, ok := raw["mode"]; ok {
		if err := json.Unmarshal(v, &b.Mode); err != nil {
			return b, pf, err
		}
		pf.mode = true
	}
	if v, ok := raw["error_status"]; ok {
		if err := json.Unmarshal(v, &b.ErrorStatus); err != nil {
			return b, pf, err
		}
		pf.errorStatus = true
	}
	if v, ok := raw["error_rate"]; ok {
		if err := json.Unmarshal(v, &b.ErrorRate); err != nil {
			return b, pf, err
		}
		pf.errorRate = true
	}
