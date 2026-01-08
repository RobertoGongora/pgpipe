package migration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/testutil"
)

// mockMySQLClient is a simple mock for testing migration logic
type mockMySQLClient struct {
	tables   []db.TableInfo
	columns  []db.ColumnInfo
	rowCount int64
	minID    int64
	maxID    int64
}

func (m *mockMySQLClient) Ping(ctx context.Context) error { return nil }
func (m *mockMySQLClient) Close() error                   { return nil }
func (m *mockMySQLClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	return m.tables, nil
}
func (m *mockMySQLClient) GetColumns(ctx context.Context, tableName string) ([]db.ColumnInfo, error) {
	return m.columns, nil
}
func (m *mockMySQLClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	for _, col := range m.columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}
func (m *mockMySQLClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	return m.rowCount, nil
}
func (m *mockMySQLClient) GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error) {
	return m.minID, m.maxID, nil
}
func (m *mockMySQLClient) FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error) {
	return nil, nil // Not used in these tests
}

// mockPostgresClient is a simple mock for testing migration logic
type mockPostgresClient struct {
	tables        []db.TableInfo
	columns       []db.ColumnInfo
	insertedCount int
}

func (m *mockPostgresClient) Ping(ctx context.Context) error { return nil }
func (m *mockPostgresClient) Close()                         {}
func (m *mockPostgresClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	return m.tables, nil
}
func (m *mockPostgresClient) GetColumns(ctx context.Context, tableName string) ([]db.ColumnInfo, error) {
	return m.columns, nil
}
func (m *mockPostgresClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	for _, col := range m.columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}
func (m *mockPostgresClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	return 0, nil
}
func (m *mockPostgresClient) InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error) {
	if m.insertedCount > 0 {
		return m.insertedCount, nil
	}
	return len(rows), nil
}

// setupTestClients creates mock MySQL and Postgres clients for testing
func setupTestClients() (db.MySQLClientInterface, db.PostgresClientInterface) {
	mysqlClient := &mockMySQLClient{
		tables:   testutil.GenerateMockTableInfo(5),
		columns:  testutil.GenerateMockColumnInfo(4, true),
		rowCount: 10000,
		minID:    1,
		maxID:    10000,
	}

	pgClient := &mockPostgresClient{
		tables:        testutil.GenerateMockTableInfo(5),
		columns:       testutil.GenerateMockColumnInfo(4, true),
		insertedCount: 1000,
	}

	return mysqlClient, pgClient
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
