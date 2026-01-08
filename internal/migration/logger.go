package migration

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/pgpipe/pgpipe/internal/config"
)

var (
	debugLog     *os.File
	debugLogOnce sync.Once
	debugMu      sync.Mutex
)

// getDebugLog returns the debug log file, creating it if needed
func getDebugLog() *os.File {
	debugLogOnce.Do(func() {
		logDir := filepath.Join(config.ConfigDir, config.LogsDir)
		os.MkdirAll(logDir, 0755)

		logPath := filepath.Join(logDir, "debug.log")
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// If we can't open the log file, use a no-op writer
			debugLog = nil
			return
		}
		debugLog = f
	})
	return debugLog
}

// logDebug writes a debug message to the log file
func logDebug(format string, args ...interface{}) {
	debugMu.Lock()
	defer debugMu.Unlock()

	log := getDebugLog()
	if log == nil {
		return
	}

	fmt.Fprintf(log, format+"\n", args...)
}

// LogDebug writes a debug message to the log file (exported for use in other packages)
func LogDebug(format string, args ...interface{}) {
	logDebug(format, args...)
}

// CloseDebugLog closes the debug log file
func CloseDebugLog() {
	debugMu.Lock()
	defer debugMu.Unlock()

	if debugLog != nil {
		debugLog.Close()
		debugLog = nil
	}
}

// DiscardDebugLog disables debug logging completely
func DiscardDebugLog() {
	debugMu.Lock()
	defer debugMu.Unlock()

	if debugLog != nil {
		debugLog.Close()
	}
	debugLog = &os.File{}
	debugLog.Close()
	// Set to a no-op writer
	var devNull *os.File
	devNull, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	debugLog = devNull
}

// GetDebugWriter returns an io.Writer for debug output
func GetDebugWriter() io.Writer {
	log := getDebugLog()
	if log == nil {
		return io.Discard
	}
	return log
}
