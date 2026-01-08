package tui

import (
	"testing"
)

// TestStartMigrationReturnsCommand tests that startMigration returns a command that produces
// MigrationStartedMsg. This is critical for the UI update loop to work properly.
//
// The proper Bubble Tea pattern:
// 1. startMigration() returns tea.Cmd (function)
// 2. Command produces MigrationStartedMsg with the migrator
// 3. Update() receives message and sets m.migrator
// 4. Update() schedules first tick
// 5. Tick loop continues
func TestStartMigrationReturnsCommand(t *testing.T) {
	model := createTestModel()
	model.screen = ScreenSettings

	// Call startMigration - should return a tea.Cmd (function)
	cmd := model.startMigration()

	// Verify it's not nil
	if cmd == nil {
		t.Fatal("startMigration() returned nil - UI will freeze!")
	}

	// Execute the command to get the message
	msg := cmd()

	// Verify the message is not nil
	if msg == nil {
		t.Fatal("Command returned nil message")
	}

	// Verify it returns MigrationStartedMsg (not TickMsg directly)
	startedMsg, ok := msg.(MigrationStartedMsg)
	if !ok {
		t.Fatalf("Expected MigrationStartedMsg, got %T", msg)
	}

	// Verify the migrator is included in the message
	if startedMsg.migrator == nil {
		t.Fatal("MigrationStartedMsg.migrator is nil - migration won't run")
	}

	// Verify the state is included
	if startedMsg.state == nil {
		t.Fatal("MigrationStartedMsg.state is nil - migration state missing")
	}

	// Verify the done channel is included
	if startedMsg.done == nil {
		t.Fatal("MigrationStartedMsg.done channel is nil - completion won't be signaled")
	}
}
