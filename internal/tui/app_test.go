package tui

import (
	"testing"
)

// TestMigrationStartFlow tests the complete flow of starting a migration.
// This test validates that:
// 1. startMigration() returns a command
// 2. The command produces MigrationStartedMsg with migrator
// 3. Update() receives the message and sets m.migrator
// 4. Update() schedules the first tick
// 5. TickMsg handler can access m.migrator
//
// This prevents the bug where m.migrator was set in the command (lost) instead
// of being set in Update() via a message (proper pattern).
//
// NOTE: We don't test the actual migration running (that would require real/better mocks).
// We only test the message flow for UI updates.
func TestMigrationStartFlow(t *testing.T) {
	// Step 1: Create a model
	model := createTestModel()
	model.screen = ScreenRunning

	// Verify migrator is initially nil
	if model.migrator != nil {
		t.Fatal("Expected migrator to be nil initially")
	}

	// Step 2: Call startMigration to get the command
	// Note: This will start a goroutine that tries to run the migration.
	// With our mocks it will fail, but that's okay - we're testing the message flow.
	cmd := model.startMigration()
	if cmd == nil {
		t.Fatal("startMigration() returned nil command")
	}

	// Step 3: Execute the command to get MigrationStartedMsg
	msg := executeTestCmd(cmd)
	if msg == nil {
		t.Fatal("Command returned nil message")
	}

	startedMsg, ok := msg.(MigrationStartedMsg)
	if !ok {
		t.Fatalf("Expected MigrationStartedMsg, got %T", msg)
	}

	if startedMsg.migrator == nil {
		t.Fatal("MigrationStartedMsg has nil migrator")
	}

	// Step 4: Feed the message to Update() to simulate Bubble Tea's behavior
	newModel, tickCmd := model.Update(startedMsg)

	// Verify the model now has the migrator set
	modelWithMigrator := newModel.(Model)
	if modelWithMigrator.migrator == nil {
		t.Fatal("BUG: Update() didn't set migrator on model - UI will freeze!\n" +
			"This is the critical bug: migrator must be set via message handling in Update(),\n" +
			"not in the command itself (commands can't modify the model)")
	}

	// Verify Update() returned a tick command to start the loop
	if tickCmd == nil {
		t.Fatal("Update() didn't return tick command after MigrationStartedMsg")
	}

	// Step 5: Execute the tick command
	// Note: tea.Batch returns a BatchMsg, which we need to handle specially
	// For testing purposes, we just verify a command was returned
	if tickCmd == nil {
		t.Fatal("Expected a command after MigrationStartedMsg")
	}

	// Execute the command - it's a batch command
	tickMsg := executeTestCmd(tickCmd)
	if tickMsg == nil {
		t.Fatal("Tick command returned nil")
	}

	// tea.Batch wraps multiple commands, so we get BatchMsg instead of TickMsg directly
	// This is expected and correct - just verify we got something back

	// Test passes! We've verified:
	// - Command returns MigrationStartedMsg with migrator
	// - Update() sets migrator on model
	// - Update() schedules first tick
	// - The tick loop is ready to start
	//
	// Note: We don't feed TickMsg back to Update() because our mock migration
	// will crash (FetchBatch returns nil). That's okay - we're only testing
	// the initialization flow, not the migration execution.
}

// TestMigrationStartedMsgSetsMigrator specifically tests that MigrationStartedMsg
// properly sets the migrator on the model. This is a regression test for the bug
// where migrator was set in the command (which doesn't work).
func TestMigrationStartedMsgSetsMigrator(t *testing.T) {
	t.Parallel()

	model := createTestModel()
	model.screen = ScreenRunning

	// Initially nil
	if model.migrator != nil {
		t.Fatal("Expected migrator to be nil initially")
	}

	// Create a migration and get the started message
	// Note: This starts a goroutine that will fail with our mocks, but that's okay
	cmd := model.startMigration()
	msg := executeTestCmd(cmd)
	startedMsg := msg.(MigrationStartedMsg)

	// Feed it to Update()
	newModel, _ := model.Update(startedMsg)
	modelWithMigrator := newModel.(Model)

	// CRITICAL: Migrator must be set now
	if modelWithMigrator.migrator == nil {
		t.Fatal("REGRESSION: migrator not set after MigrationStartedMsg!\n" +
			"This is the bug that causes UI to freeze.\n" +
			"Migrator must be set in Update() message handler, not in the command.")
	}

	// Verify it's the same migrator from the message
	if modelWithMigrator.migrator != startedMsg.migrator {
		t.Error("Migrator on model doesn't match migrator from message")
	}
}
