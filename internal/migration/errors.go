package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
)

// ErrorEntry represents a single error log entry
type ErrorEntry struct {
	MySQLID    int64     `json:"mysql_id"`
	Error      string    `json:"error"`
	RawPreview string    `json:"raw_preview,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// ErrorLogger handles writing errors to a JSONL file
type ErrorLogger struct {
	file   *os.File
	mu     sync.Mutex
	path   string
	count  int
	recent []ErrorEntry
}

const maxRecentErrors = 10
const maxPreviewLength = 100

// NewErrorLogger creates a new error logger for the given session
func NewErrorLogger(sessionID string) (*ErrorLogger, error) {
	if err := config.EnsureConfigDir(); err != nil {
		return nil, err
	}

	logPath := filepath.Join(config.ConfigDir, config.LogsDir, sessionID+"_errors.jsonl")

	// Open file in append mode, create if doesn't exist
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open error log: %w", err)
	}

	return &ErrorLogger{
		file:   file,
		path:   logPath,
		recent: make([]ErrorEntry, 0, maxRecentErrors),
	}, nil
}

// Log writes an error entry to the log file
func (l *ErrorLogger) Log(mysqlID int64, err error, rawValue string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Truncate raw value for preview
	preview := rawValue
	if len(preview) > maxPreviewLength {
		preview = preview[:maxPreviewLength] + "..."
	}

	entry := ErrorEntry{
		MySQLID:    mysqlID,
		Error:      err.Error(),
		RawPreview: preview,
		Timestamp:  time.Now(),
	}

	// Write to file
	data, jsonErr := json.Marshal(entry)
	if jsonErr != nil {
		return fmt.Errorf("failed to marshal error entry: %w", jsonErr)
	}

	if _, writeErr := l.file.Write(append(data, '\n')); writeErr != nil {
		return fmt.Errorf("failed to write error entry: %w", writeErr)
	}

	// Track recent errors
	l.count++
	if len(l.recent) >= maxRecentErrors {
		l.recent = l.recent[1:]
	}
	l.recent = append(l.recent, entry)

	return nil
}

// Close closes the error log file
func (l *ErrorLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// Path returns the path to the error log file
func (l *ErrorLogger) Path() string {
	return l.path
}

// Count returns the number of errors logged
func (l *ErrorLogger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.count
}

// RecentErrors returns the most recent errors
func (l *ErrorLogger) RecentErrors() []ErrorEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]ErrorEntry, len(l.recent))
	copy(result, l.recent)
	return result
}

// LoadErrorCount counts errors in an existing log file
func LoadErrorCount(logPath string) (int, error) {
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	count := 0
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var entry ErrorEntry
		if err := decoder.Decode(&entry); err == nil {
			count++
		}
	}

	return count, nil
}
