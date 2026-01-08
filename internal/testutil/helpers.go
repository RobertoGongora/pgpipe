package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Common test errors
var (
	ErrMockError = errors.New("mock error for testing")
)

// CleanupTestState removes test state files
func CleanupTestState(t *testing.T) {
	t.Helper()
	// Use temp directory for test state
	testDir := filepath.Join(os.TempDir(), "pgpipe-test")
	if err := os.RemoveAll(testDir); err != nil {
		t.Logf("Warning: failed to cleanup test directory: %v", err)
	}
}

// SetupTestDir creates a temporary test directory
func SetupTestDir(t *testing.T) string {
	t.Helper()
	testDir := filepath.Join(os.TempDir(), "pgpipe-test", t.Name())
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	return testDir
}

// TeardownTestDir removes a test directory
func TeardownTestDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.RemoveAll(dir); err != nil {
		t.Logf("Warning: failed to remove test directory: %v", err)
	}
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected an error, got nil")
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}) {
	t.Helper()
	if expected != actual {
		t.Fatalf("Expected %v, got %v", expected, actual)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("Assertion failed: %s", msg)
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("Assertion failed: %s", msg)
	}
}

// AssertGreater fails the test if actual <= expected
func AssertGreater(t *testing.T, actual, expected int64, msg string) {
	t.Helper()
	if actual <= expected {
		t.Fatalf("%s: expected %d > %d", msg, actual, expected)
	}
}
