package migration

import (
	"fmt"
	"testing"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/testutil"
)

// setupTestClients creates mock MySQL and Postgres clients for testing
func setupTestClients() (db.MySQLClientInterface, db.PostgresClientInterface) {
	return testutil.NewMockMySQLClient(), testutil.NewMockPostgresClient()
}

// createTestState creates a test migration state
func createTestState(totalRows int64, sessionID string) *State {
	if sessionID == "" {
		sessionID = time.Now().Format("2006-01-02_15-04-05")
	}

	return &State{
		ConfigHash: "test-hash-123",
		Session: SessionState{
			ID:        sessionID,
			StartedAt: time.Now(),
			ErrorLog:  fmt.Sprintf("/tmp/pgpipe-test-%s/logs/%s_errors.jsonl", sessionID, sessionID),
		},
		Source: SourceState{
			Table:      "users",
			TotalRows:  totalRows,
			PrimaryKey: "id",
			MinID:      1,
			MaxID:      totalRows,
		},
		Progress: ProgressState{
			LastCursor:    0,
			ProcessedRows: 0,
			ImportedRows:  0,
			SkippedRows:   0,
		},
		Batches: BatchState{
			Size:      1000,
			Completed: 0,
		},
		LastRun: LastRunState{
			Mode:             "continuous",
			BatchesRequested: 0,
			BatchesCompleted: 0,
			RowsThisRun:      0,
			DurationSeconds:  0,
			EndedAt:          time.Time{},
		},
	}
}

// createTestMigrationConfig creates a test migration configuration
func createTestMigrationConfig() MigrationConfig {
	return MigrationConfig{
		SourceTable:   "users",
		TargetTable:   "users",
		SourcePK:      "id",
		SourceColumns: []string{"name", "email", "created_at"},
		TargetColumns: []string{"name", "email", "created_at"},
		Mapping: []config.ColumnMapping{
			{Source: "name", Target: "name"},
			{Source: "email", Target: "email"},
			{Source: "created_at", Target: "created_at"},
		},
		BatchSize:  1000,
		Mode:       RunModeContinuous,
		BatchLimit: 0,
	}
}

func TestMigratorCreation(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	if migrator == nil {
		t.Fatal("Expected migrator to be created")
	}

	if migrator.mysql == nil {
		t.Error("Expected mysql client to be set")
	}

	if migrator.postgres == nil {
		t.Error("Expected postgres client to be set")
	}

	if migrator.state == nil {
		t.Error("Expected state to be set")
	}
}

func TestMigratorStop(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	// Stop should not panic
	migrator.Stop()

	if !migrator.stopped {
		t.Error("Expected migrator to be marked as stopped")
	}
}

func TestStateUpdates(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")

	// Test UpdateAfterBatch
	state.UpdateAfterBatch(1000, 1000, 950, 50)

	testutil.AssertEqual(t, int64(1000), state.Progress.LastCursor)
	testutil.AssertEqual(t, int64(1000), state.Progress.ProcessedRows)
	testutil.AssertEqual(t, int64(950), state.Progress.ImportedRows)
	testutil.AssertEqual(t, int64(50), state.Progress.SkippedRows)
	testutil.AssertEqual(t, 1, state.Batches.Completed)
}

func TestProgressPercentage(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")
	state.Progress.ProcessedRows = 5000

	percent := state.ProgressPercent()
	if percent != 50.0 {
		t.Errorf("Expected 50%%, got %.2f%%", percent)
	}
}

func TestIsComplete(t *testing.T) {
	t.Parallel()

	state := createTestState(10000, "test-session")

	// Not complete initially
	if state.IsComplete() {
		t.Error("Expected migration to not be complete")
	}

	// Complete after reaching max ID
	state.Progress.LastCursor = 10000
	if !state.IsComplete() {
		t.Error("Expected migration to be complete")
	}
}

func TestTransformRow(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session")

	cfg := MigrationConfig{
		SourceTable:   "users",
		TargetTable:   "users",
		SourcePK:      "id",
		SourceColumns: []string{"name", "email"},
		TargetColumns: []string{"name", "email"},
		Mapping: []config.ColumnMapping{
			{Source: "name", Target: "name"},
			{Source: "email", Target: "email"},
		},
		BatchSize: 1000,
		Mode:      RunModeContinuous,
	}

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	// Create mock scan data (simulating sql.Rows.Scan results)
	pkVal := int64(123)
	nameVal := interface{}("John Doe")
	emailVal := interface{}("john@example.com")

	scanDest := []interface{}{
		&pkVal,
		&nameVal,
		&emailVal,
	}

	result, err := migrator.transformRow(scanDest, pkVal)
	testutil.AssertNoError(t, err)

	if len(result) != 2 {
		t.Fatalf("Expected 2 columns in result, got %d", len(result))
	}

	if result[0] != "John Doe" {
		t.Errorf("Expected name 'John Doe', got %v", result[0])
	}

	if result[1] != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %v", result[1])
	}
}

func TestApplyTransformIntToBool(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session")

	cfg := MigrationConfig{
		SourceTable:   "users",
		TargetTable:   "users",
		SourcePK:      "id",
		SourceColumns: []string{"name"},
		TargetColumns: []string{"name"},
		Mapping: []config.ColumnMapping{
			{Source: "name", Target: "name"},
		},
		BatchSize: 1000,
		Mode:      RunModeContinuous,
	}

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	tests := []struct {
		name      string
		input     interface{}
		expected  interface{}
		expectErr bool
	}{
		{"int64 zero is false", int64(0), false, false},
		{"int64 one is true", int64(1), true, false},
		{"int64 non-zero is true", int64(5), true, false},
		{"int32 one is true", int32(1), true, false},
		{"int zero is false", int(0), false, false},
		{"bool true passthrough", true, true, false},
		{"bool false passthrough", false, false, false},
		{"[]byte 1 is true", []byte("1"), true, false},
		{"[]byte 0 is false", []byte("0"), false, false},
		{"nil returns nil", nil, nil, false},
		{"unexpected type errors", "oops", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrator.applyTransform(tt.input, "int_to_bool", int64(1))
			if tt.expectErr {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}
			testutil.AssertNoError(t, err)
			if result != tt.expected {
				t.Errorf("Expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestApplyTransformStringToUuid(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session-uuid")

	cfg := MigrationConfig{
		SourceTable:   "users",
		TargetTable:   "users",
		SourcePK:      "id",
		SourceColumns: []string{"name"},
		TargetColumns: []string{"name"},
		Mapping: []config.ColumnMapping{
			{Source: "name", Target: "name"},
		},
		BatchSize: 1000,
		Mode:      RunModeContinuous,
	}

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	const validUUID = "550e8400-e29b-41d4-a716-446655440000"

	tests := []struct {
		name      string
		input     interface{}
		expected  interface{}
		expectErr bool
	}{
		{"[]byte UUID becomes string", []byte(validUUID), validUUID, false},
		{"string UUID passthrough", validUUID, validUUID, false},
		{"nil returns nil", nil, nil, false},
		{"unexpected type errors", int64(42), nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrator.applyTransform(tt.input, "string_to_uuid", int64(1))
			if tt.expectErr {
				if err == nil {
					t.Fatalf("Expected error, got nil")
				}
				return
			}
			testutil.AssertNoError(t, err)
			if result != tt.expected {
				t.Errorf("Expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
			}
		})
	}
}
