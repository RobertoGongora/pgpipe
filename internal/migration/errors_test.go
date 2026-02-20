package migration

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pgpipe/pgpipe/internal/testutil"
)

func TestErrorLoggerLifecycle(t *testing.T) {
	// Use a temp dir as the working directory so EnsureConfigDir writes there.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	logger, err := NewErrorLogger("test-session-lifecycle")
	testutil.AssertNoError(t, err)

	if logger == nil {
		t.Fatal("NewErrorLogger() returned nil")
	}

	// Verify log file was created
	logPath := logger.Path()
	if logPath == "" {
		t.Fatal("logger.Path() returned empty string")
	}
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatalf("log file not created at %s", logPath)
	}

	// Count starts at zero
	if logger.Count() != 0 {
		t.Errorf("Count() = %d, want 0 before any logs", logger.Count())
	}

	// Log some errors
	err1 := errors.New("invalid JSON")
	err2 := errors.New("null constraint violation")
	err3 := errors.New("uuid format error")

	testutil.AssertNoError(t, logger.Log(1, err1, `{"bad": json}`))
	testutil.AssertNoError(t, logger.Log(2, err2, "some raw value"))
	testutil.AssertNoError(t, logger.Log(3, err3, "550e8400-INVALID"))

	// Count should be 3
	if logger.Count() != 3 {
		t.Errorf("Count() = %d, want 3", logger.Count())
	}

	// Recent errors
	recent := logger.RecentErrors()
	if len(recent) != 3 {
		t.Fatalf("RecentErrors() len = %d, want 3", len(recent))
	}
	if recent[0].MySQLID != 1 {
		t.Errorf("recent[0].MySQLID = %d, want 1", recent[0].MySQLID)
	}
	if recent[0].Error != "invalid JSON" {
		t.Errorf("recent[0].Error = %q, want %q", recent[0].Error, "invalid JSON")
	}

	// Close
	testutil.AssertNoError(t, logger.Close())
}

func TestErrorLoggerPreviewTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	logger, err := NewErrorLogger("test-session-truncation")
	testutil.AssertNoError(t, err)
	defer logger.Close()

	// Build a value longer than 100 chars
	longValue := strings.Repeat("x", 200)
	testutil.AssertNoError(t, logger.Log(99, errors.New("truncation test"), longValue))

	recent := logger.RecentErrors()
	if len(recent) != 1 {
		t.Fatalf("Expected 1 recent error, got %d", len(recent))
	}

	preview := recent[0].RawPreview
	// Should be truncated to maxPreviewLength + "..."
	if len(preview) > maxPreviewLength+3 {
		t.Errorf("RawPreview length = %d, want at most %d (with ...)", len(preview), maxPreviewLength+3)
	}
	if !strings.HasSuffix(preview, "...") {
		t.Errorf("RawPreview should end with '...', got %q", preview)
	}
}

func TestErrorLoggerRecentCap(t *testing.T) {
	// Verify that recent errors are capped at maxRecentErrors (10)
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	logger, err := NewErrorLogger("test-session-cap")
	testutil.AssertNoError(t, err)
	defer logger.Close()

	for i := 0; i < 15; i++ {
		testutil.AssertNoError(t, logger.Log(int64(i), errors.New("error"), "raw"))
	}

	if logger.Count() != 15 {
		t.Errorf("Count() = %d, want 15", logger.Count())
	}

	recent := logger.RecentErrors()
	if len(recent) > maxRecentErrors {
		t.Errorf("RecentErrors() len = %d, exceeds cap of %d", len(recent), maxRecentErrors)
	}

	// The last entry should have MySQLID = 14 (0-indexed)
	if recent[len(recent)-1].MySQLID != 14 {
		t.Errorf("last recent error MySQLID = %d, want 14", recent[len(recent)-1].MySQLID)
	}
}

func TestLoadErrorCount(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create a logger and write some entries
	logger, err := NewErrorLogger("test-session-load-count")
	testutil.AssertNoError(t, err)

	for i := 0; i < 5; i++ {
		testutil.AssertNoError(t, logger.Log(int64(i+1), errors.New("error"), "raw"))
	}
	testutil.AssertNoError(t, logger.Close())

	// Load the count from disk
	count, err := LoadErrorCount(logger.Path())
	testutil.AssertNoError(t, err)
	if count != 5 {
		t.Errorf("LoadErrorCount() = %d, want 5", count)
	}
}

func TestLoadErrorCountMissingFile(t *testing.T) {
	t.Parallel()

	count, err := LoadErrorCount("/tmp/pgpipe-nonexistent-errors-99999.jsonl")
	testutil.AssertNoError(t, err)
	if count != 0 {
		t.Errorf("LoadErrorCount() on missing file = %d, want 0", count)
	}
}

func TestErrorLoggerPathContainsSessionID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	sessionID := "my-unique-session-id"
	logger, err := NewErrorLogger(sessionID)
	testutil.AssertNoError(t, err)
	defer logger.Close()

	if !strings.Contains(logger.Path(), sessionID) {
		t.Errorf("logger.Path() = %q should contain session ID %q", logger.Path(), sessionID)
	}
	if !strings.HasSuffix(logger.Path(), "_errors.jsonl") {
		t.Errorf("logger.Path() = %q should end with _errors.jsonl", logger.Path())
	}
}

func TestErrorLoggerFileHasContent(t *testing.T) {
	// Verify JSONL content was actually written to disk
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	logger, err := NewErrorLogger("test-session-content")
	testutil.AssertNoError(t, err)

	testutil.AssertNoError(t, logger.Log(42, errors.New("disk write test"), "raw value"))
	testutil.AssertNoError(t, logger.Close())

	data, err := os.ReadFile(logger.Path())
	testutil.AssertNoError(t, err)

	content := string(data)
	if !strings.Contains(content, `"mysql_id":42`) {
		t.Errorf("log file content should contain mysql_id 42, got: %s", content)
	}
	if !strings.Contains(content, "disk write test") {
		t.Errorf("log file content should contain error message, got: %s", content)
	}
}

func TestStatePathForConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configPath string
		expected   string
	}{
		{
			name:       "simple path",
			configPath: "configs/individuals.yaml",
			expected:   filepath.Join("configs", ".individuals.state.yaml"),
		},
		{
			name:       "root path",
			configPath: "foo.yaml",
			expected:   ".foo.state.yaml",
		},
		{
			name:       "nested path",
			configPath: "a/b/c/table.yaml",
			expected:   filepath.Join("a", "b", "c", ".table.state.yaml"),
		},
		{
			name:       "hidden file",
			configPath: "configs/.hidden.yaml",
			expected:   filepath.Join("configs", "..hidden.state.yaml"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StatePathForConfig(tt.configPath)
			if result != tt.expected {
				t.Errorf("StatePathForConfig(%q) = %q, want %q", tt.configPath, result, tt.expected)
			}
		})
	}
}

func TestLoadStateFromPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Create and save state to a custom path
	state := createTestState(5000, "session-path-test")
	state.Progress.ProcessedRows = 1234
	state.Progress.LastCursor = 1234

	customPath := filepath.Join(tmpDir, ".mytable.state.yaml")
	state.SetStatePath(customPath)
	testutil.AssertNoError(t, state.Save())

	// Load from the same custom path
	loaded, err := LoadStateFromPath(customPath)
	testutil.AssertNoError(t, err)

	if loaded == nil {
		t.Fatal("LoadStateFromPath() returned nil for existing file")
	}
	if loaded.Progress.ProcessedRows != 1234 {
		t.Errorf("ProcessedRows = %d, want 1234", loaded.Progress.ProcessedRows)
	}
	if loaded.statePath != customPath {
		t.Errorf("statePath = %q, want %q", loaded.statePath, customPath)
	}
}

func TestLoadStateFromPathMissing(t *testing.T) {
	t.Parallel()

	state, err := LoadStateFromPath("/tmp/pgpipe-nonexistent-state-99999.yaml")
	testutil.AssertNoError(t, err)
	if state != nil {
		t.Error("LoadStateFromPath() should return nil for missing file")
	}
}

func TestSetStatePath(t *testing.T) {
	t.Parallel()

	state := createTestState(1000, "set-path-test")
	customPath := "/tmp/pgpipe-test-custom-state.yaml"
	state.SetStatePath(customPath)

	resolved := state.resolveSavePath()
	if resolved != customPath {
		t.Errorf("resolveSavePath() = %q, want %q", resolved, customPath)
	}
}
