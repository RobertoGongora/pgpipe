package migration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/testutil"
)

func TestNewState(t *testing.T) {
	t.Parallel()

	state := NewState("test-hash")

	if state.ConfigHash != "test-hash" {
		t.Errorf("Expected config hash 'test-hash', got %s", state.ConfigHash)
	}

	if state.Session.ID == "" {
		t.Error("Expected session ID to be set")
	}

	if state.Session.StartedAt.IsZero() {
		t.Error("Expected start time to be set")
	}
}

func TestStateSaveAndLoad(t *testing.T) {
	// Note: This test modifies the real .pgpipe directory
	// We clean up after ourselves

	// Create a state
	state := createTestState(10000, "test-session-save-load")
	state.Progress.ProcessedRows = 5000
	state.Progress.LastCursor = 5000
	state.Batches.Completed = 5

	// Save it
	err := state.Save()
	testutil.AssertNoError(t, err)

	// Cleanup at end
	defer DeleteState()

	// Verify file exists
	statePath := filepath.Join(config.ConfigDir, config.StateFile)
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load it back
	loadedState, err := LoadState()
	testutil.AssertNoError(t, err)

	// Verify data matches
	testutil.AssertEqual(t, state.ConfigHash, loadedState.ConfigHash)
	testutil.AssertEqual(t, state.Progress.ProcessedRows, loadedState.Progress.ProcessedRows)
	testutil.AssertEqual(t, state.Progress.LastCursor, loadedState.Progress.LastCursor)
	testutil.AssertEqual(t, state.Batches.Completed, loadedState.Batches.Completed)
}

func TestStateUpdateAfterBatch(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")

	// Process first batch
	state.UpdateAfterBatch(1000, 1000, 950, 50)

	testutil.AssertEqual(t, int64(1000), state.Progress.LastCursor)
	testutil.AssertEqual(t, int64(1000), state.Progress.ProcessedRows)
	testutil.AssertEqual(t, int64(950), state.Progress.ImportedRows)
	testutil.AssertEqual(t, int64(50), state.Progress.SkippedRows)
	testutil.AssertEqual(t, 1, state.Batches.Completed)

	// Process second batch
	state.UpdateAfterBatch(2000, 1000, 980, 20)

	testutil.AssertEqual(t, int64(2000), state.Progress.LastCursor)
	testutil.AssertEqual(t, int64(2000), state.Progress.ProcessedRows)
	testutil.AssertEqual(t, int64(1930), state.Progress.ImportedRows)
	testutil.AssertEqual(t, int64(70), state.Progress.SkippedRows)
	testutil.AssertEqual(t, 2, state.Batches.Completed)
}

func TestProgressPercentCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		totalRows     int64
		processedRows int64
		expectedPct   float64
	}{
		{"0%", 10000, 0, 0.0},
		{"25%", 10000, 2500, 25.0},
		{"50%", 10000, 5000, 50.0},
		{"75%", 10000, 7500, 75.0},
		{"100%", 10000, 10000, 100.0},
		{"Empty table", 0, 0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := createTestState(tt.totalRows, "test-session")
			state.Progress.ProcessedRows = tt.processedRows

			pct := state.ProgressPercent()
			if pct != tt.expectedPct {
				t.Errorf("Expected %.2f%%, got %.2f%%", tt.expectedPct, pct)
			}
		})
	}
}

func TestRemainingRows(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")
	state.Progress.ProcessedRows = 3000

	remaining := state.RemainingRows()
	testutil.AssertEqual(t, int64(7000), remaining)
}

func TestEstimatedBatchesRemaining(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")
	state.Batches.Size = 1000
	state.Progress.ProcessedRows = 3000

	remaining := state.EstimatedBatchesRemaining()
	testutil.AssertEqual(t, 7, remaining)
}

func TestIsCompleteBoundary(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")

	// Just before completion
	state.Progress.LastCursor = 9999
	if state.IsComplete() {
		t.Error("Expected migration to not be complete at cursor 9999")
	}

	// At completion
	state.Progress.LastCursor = 10000
	if !state.IsComplete() {
		t.Error("Expected migration to be complete at cursor 10000")
	}

	// Past completion
	state.Progress.LastCursor = 10001
	if !state.IsComplete() {
		t.Error("Expected migration to be complete at cursor 10001")
	}
}

func TestStartAndEndRun(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")

	// Start a new run
	state.StartNewRun("continuous", 0)

	testutil.AssertEqual(t, "continuous", state.LastRun.Mode)
	testutil.AssertEqual(t, 0, state.LastRun.BatchesRequested)
	testutil.AssertEqual(t, 0, state.LastRun.BatchesCompleted)
	testutil.AssertEqual(t, int64(0), state.LastRun.RowsThisRun)

	// Simulate some progress
	state.UpdateAfterBatch(1000, 1000, 1000, 0)

	// End the run
	duration := 5 * time.Second
	state.EndRun(duration)

	if state.LastRun.DurationSeconds != 5.0 {
		t.Errorf("Expected duration 5.0 seconds, got %.2f", state.LastRun.DurationSeconds)
	}

	if state.LastRun.EndedAt.IsZero() {
		t.Error("Expected end time to be set")
	}
}

func TestDeleteState(t *testing.T) {
	// Create and save a state
	state := createTestState(10000, "test-session-delete")
	err := state.Save()
	testutil.AssertNoError(t, err)

	// Verify it exists
	if !StateExists() {
		t.Fatal("State file should exist")
	}

	// Delete it
	err = DeleteState()
	testutil.AssertNoError(t, err)

	// Verify it's gone
	if StateExists() {
		t.Error("State file should not exist after deletion")
	}
}

func TestLoadNonExistentState(t *testing.T) {
	// Ensure no state exists first
	DeleteState()

	// Try to load non-existent state
	state, err := LoadState()
	testutil.AssertNoError(t, err)

	if state != nil {
		t.Error("Expected nil state when file doesn't exist")
	}
}
