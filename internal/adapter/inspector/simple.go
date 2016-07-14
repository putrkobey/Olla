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
