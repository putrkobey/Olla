package inspector

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/putrkobey/Olla/internal/logger"
)

// Entry represents a single request or response entry in the inspector log
type Entry struct {
	Type      string          `json:"type"` // "request" or "response"
	Timestamp string          `json:"ts"`   // ISO8601 timestamp
	Model     string          `json:"model,omitempty"`
	Body      json.RawMessage `json:"body"`
}

// Simple is a minimal inspector that dumps requests/responses to disk
// Designed for quick debugging without fancy features
type Simple struct {
	logger        logger.StyledLogger
	outputDir     string
	sessionHeader string
	warnOnce      sync.Once  // ensures security warning is logged only once
	mu            sync.Mutex // protects file operations
	enabled       bool
	warningLogged bool // indicator for tests only; not used for control flow
}

const (
	// maxSessionIDLength limits session ID length to prevent filesystem issues
	maxSessionIDLength = 64
	// defaultSessionID is used when session ID is invalid or empty
	defaultSessionID = "default"
)

// validSessionIDPattern matches only safe characters for filenames
// Allows alphanumeric, dash, and underscore only
var validSessionIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// NewSimple creates a new simple inspector
func NewSimple(enabled bool, outputDir, sessionHeader string, log logger.StyledLogger) *Simple {
	if sessionHeader == "" {
		sessionHeader = "X-Session-ID"
	}

	return &Simple{
		enabled:       enabled,
		outputDir:     outputDir,
		sessionHeader: sessionHeader,
		logger:        log,
	}
}

// sanitiseSessionID ensures session ID is safe for use in file paths
// Prevents path traversal attacks and other filesystem exploits
func sanitiseSessionID(sessionID string, outputDir string) (string, error) {
	if sessionID == "" || sessionID == defaultSessionID {
		return defaultSessionID, nil
	}

	if len(sessionID) > maxSessionIDLength {
		return "", fmt.Errorf("session ID exceeds maximum length of %d characters", maxSessionIDLength)
	}

	// checks for null bytes (common attack vector)
	if strings.Contains(sessionID, "\x00") {
		return "", errors.New("session ID contains null bytes")
	}

	// only allow safe characters - blocks path traversal attempts
	if !validSessionIDPattern.MatchString(sessionID) {
		return "", errors.New("session ID contains invalid characters (only alphanumeric, dash, underscore allowed)")
	}

	// additional defence: verify resolved path stays within outputDir
	// this tries to catch edge cases even if regex missed something
	testPath := filepath.Join(outputDir, "2006-01-02", sessionID+".jsonl")
	absTestPath, err := filepath.Abs(testPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve output directory: %w", err)
	}

	// try and ensure the resolved path is within the output directory
	// uses Clean to normalise paths and prevent ../ bypasses
	if !strings.HasPrefix(filepath.Clean(absTestPath), filepath.Clean(absOutputDir)) {
		return "", errors.New("session ID would escape output directory")
	}

	return sessionID, nil
}

// logSecurityWarning logs a prominent warning about inspector usage (once)
// Warns users that sensitive data is being written to disk
func (s *Simple) logSecurityWarning() {
	// for tests only - indicates warning was logged
	s.warningLogged = true
	s.warnOnce.Do(func() {
		s.logger.Warn("Anthropic Inspector Enabled, may log sensitive data to disk - DO NOT use in production",
			"output_directory", s.outputDir,
			"session_header", s.sessionHeader)
	})
}

// LogRequest logs an incoming request
func (s *Simple) LogRequest(sessionID, model string, body []byte) error {
	if !s.enabled {
		return nil
	}

	s.logSecurityWarning()

	entry := Entry{
		Type:      "request",
		Timestamp: time.Now().Format(time.RFC3339),
		Model:     model,
		Body:      json.RawMessage(body),
	}

	return s.writeEntry(sessionID, entry)
}

// LogResponse logs an outgoing response
func (s *Simple) LogResponse(sessionID string, body []byte) error {
	if !s.enabled {
		return nil
	}

	s.logSecurityWarning()
