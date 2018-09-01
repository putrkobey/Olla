package inspector

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/putrkobey/Olla/internal/logger"
)

func TestSimpleInspector(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Use a simple logger for tests
	log := logger.NewPlainStyledLogger(slog.Default())
	inspector := NewSimple(true, tmpDir, "X-Session-ID", log)

	t.Run("LogRequest", func(t *testing.T) {
		sessionID := "test-session"
		model := "test-model"
		body := []byte(`{"test":"request"}`)

		err := inspector.LogRequest(sessionID, model, body)
		if err != nil {
			t.Fatalf("LogRequest failed: %v", err)
		}

		// Verify file was created
		today := time.Now().Format("2006-01-02")
		filePath := filepath.Join(tmpDir, today, sessionID+".jsonl")

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("Expected file %s to exist", filePath)
		}

		// Read and verify content
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatalf("Failed to unmarshal entry: %v", err)
		}

		if entry.Type != "request" {
			t.Errorf("Expected type 'request', got %s", entry.Type)
		}
		if entry.Model != model {
			t.Errorf("Expected model %s, got %s", model, entry.Model)
		}
	})

	t.Run("LogResponse", func(t *testing.T) {
		sessionID := "test-session-2"
		body := []byte(`{"test":"response"}`)

		err := inspector.LogResponse(sessionID, body)
		if err != nil {
			t.Fatalf("LogResponse failed: %v", err)
		}

		// Verify file was created
		today := time.Now().Format("2006-01-02")
		filePath := filepath.Join(tmpDir, today, sessionID+".jsonl")

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			t.Fatalf("Failed to unmarshal entry: %v", err)
		}

		if entry.Type != "response" {
			t.Errorf("Expected type 'response', got %s", entry.Type)
		}
	})

	t.Run("MultipleEntries", func(t *testing.T) {
		sessionID := "multi-test"

		// Log request
		reqBody := []byte(`{"test":"request1"}`)
		if err := inspector.LogRequest(sessionID, "model1", reqBody); err != nil {
			t.Fatalf("LogRequest failed: %v", err)
		}

		// Log response
		respBody := []byte(`{"test":"response1"}`)
		if err := inspector.LogResponse(sessionID, respBody); err != nil {
			t.Fatalf("LogResponse failed: %v", err)
		}

		// Verify both entries are in the file
		today := time.Now().Format("2006-01-02")
		filePath := filepath.Join(tmpDir, today, sessionID+".jsonl")

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		// Count lines (each entry is a line)
		lines := 0
		for _, b := range data {
			if b == '\n' {
				lines++
			}
		}

		if lines != 2 {
			t.Errorf("Expected 2 lines, got %d", lines)
		}
	})

	t.Run("DisabledInspector", func(t *testing.T) {
		disabledInspector := NewSimple(false, tmpDir, "X-Session-ID", log)

		err := disabledInspector.LogRequest("test", "model", []byte(`{}`))
		if err != nil {
			t.Fatalf("Disabled inspector should not return error: %v", err)
		}

		// Verify no files were created for disabled inspector
		today := time.Now().Format("2006-01-02")
		filePath := filepath.Join(tmpDir, today, "test.jsonl")

		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Errorf("Expected no file when inspector is disabled")
		}
	})
}

func TestSimpleInspectorHelpers(t *testing.T) {
	log := logger.NewPlainStyledLogger(slog.Default())

	t.Run("GetSessionHeader", func(t *testing.T) {
		inspector := NewSimple(true, "/tmp", "X-Custom-Session", log)
		if got := inspector.GetSessionHeader(); got != "X-Custom-Session" {
			t.Errorf("Expected X-Custom-Session, got %s", got)
		}
	})

	t.Run("DefaultSessionHeader", func(t *testing.T) {
		inspector := NewSimple(true, "/tmp", "", log)
		if got := inspector.GetSessionHeader(); got != "X-Session-ID" {
			t.Errorf("Expected default X-Session-ID, got %s", got)
		}
	})

	t.Run("Enabled", func(t *testing.T) {
		enabledInspector := NewSimple(true, "/tmp", "", log)
		if !enabledInspector.Enabled() {
			t.Error("Expected inspector to be enabled")
		}

		disabledInspector := NewSimple(false, "/tmp", "", log)
		if disabledInspector.Enabled() {
			t.Error("Expected inspector to be disabled")
		}
	})
}

// TestSanitiseSessionID tests session ID sanitisation and path traversal prevention
func TestSanitiseSessionID(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		wantID    string
		desc      string
	}{
		{
			name:      "ValidSimpleID",
			sessionID: "test-session-123",
			wantErr:   false,
			wantID:    "test-session-123",
			desc:      "Normal alphanumeric with dashes should be allowed",
		},
		{
			name:      "ValidWithUnderscores",
			sessionID: "test_session_123",
			wantErr:   false,
			wantID:    "test_session_123",
			desc:      "Underscores should be allowed",
		},
		{
			name:      "EmptySessionID",
			sessionID: "",
			wantErr:   false,
			wantID:    "default",
			desc:      "Empty session ID should default to 'default'",
		},
		{
			name:      "DefaultSessionID",
			sessionID: "default",
			wantErr:   false,
			wantID:    "default",
			desc:      "Default session ID should be accepted",
		},
		{
			name:      "PathTraversalUnix",
			sessionID: "../../../etc/passwd",
			wantErr:   true,
			desc:      "Unix path traversal should be rejected",
		},
		{
			name:      "PathTraversalWindows",
			sessionID: "..\\..\\..\\windows\\system32",
			wantErr:   true,
			desc:      "Windows path traversal should be rejected",
		},
		{
			name:      "PathTraversalDotDot",
			sessionID: "..",
			wantErr:   true,
			desc:      "Double dot should be rejected",
		},
		{
			name:      "PathTraversalSingleDot",
			sessionID: ".",
			wantErr:   true,
			desc:      "Single dot should be rejected",
		},
		{
			name:      "URLEncodedTraversal",
			sessionID: "%2e%2e%2f%2e%2e%2ftest",
			wantErr:   true,
			desc:      "URL encoded path traversal should be rejected",
		},
		{
			name:      "NullByte",
			sessionID: "test\x00evil",
			wantErr:   true,
			desc:      "Null bytes should be rejected",
		},
		{
			name:      "NullByteAtEnd",
			sessionID: "test\x00",
			wantErr:   true,
			desc:      "Null byte at end should be rejected",
		},
		{
