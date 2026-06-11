package migration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/db"
	"github.com/RobertoGongora/pgpipe/internal/testutil"
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

func TestApplyTransformTextToJsonb(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session-jsonb")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	tests := []struct {
		name      string
		input     interface{}
		expected  interface{}
		expectErr bool
	}{
		{"valid JSON string", `{"key": "value"}`, `{"key": "value"}`, false},
		{"valid JSON array string", `[1, 2, 3]`, `[1, 2, 3]`, false},
		{"valid JSON []byte", []byte(`{"a":1}`), `{"a":1}`, false},
		{"nil returns nil", nil, nil, false},
		{"invalid JSON string", `{not valid json`, nil, true},
		{"invalid JSON []byte", []byte(`bad json`), nil, true},
		{"unexpected type (int)", int64(42), nil, true},
		{"unexpected type (bool)", true, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrator.applyTransform(tt.input, "text_to_jsonb", int64(1))
			if tt.expectErr {
				if err == nil {
					t.Fatalf("Expected error, got nil (result=%v)", result)
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

func TestApplyTransformPassthrough(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session-passthrough")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	tests := []struct {
		name      string
		transform string
		input     interface{}
	}{
		{"empty string transform with string", "", "hello"},
		{"empty string transform with int", "", int64(42)},
		{"empty string transform with nil", "", nil},
		{"none transform with string", "none", "world"},
		{"none transform with bool", "none", true},
		{"none transform with nil", "none", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := migrator.applyTransform(tt.input, tt.transform, int64(1))
			testutil.AssertNoError(t, err)
			if result != tt.input {
				t.Errorf("Passthrough transform should return input unchanged: got %v (%T), want %v (%T)",
					result, result, tt.input, tt.input)
			}
		})
	}
}

// TestApplyTransformPassthroughBytesToString locks in the text-corruption fix:
// Go's MySQL driver returns []byte for CHAR/VARCHAR/TEXT columns when scanning
// into interface{}; without converting to string, pgx.CopyFrom writes it as
// bytea (`\x<hex>`) into text columns (e.g. "Teresa" -> "\x546572657361"). The
// passthrough must convert []byte -> string.
func TestApplyTransformPassthroughBytesToString(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session-bytes")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	for _, transform := range []string{"", "none"} {
		t.Run("bytes->string transform="+transform, func(t *testing.T) {
			// "Teresa" as ASCII bytes (the real case that triggered the bug).
			result, err := migrator.applyTransform([]byte("Teresa"), transform, int64(1))
			testutil.AssertNoError(t, err)
			s, ok := result.(string)
			if !ok {
				t.Fatalf("expected string after []byte passthrough, got %T", result)
			}
			if s != "Teresa" {
				t.Errorf("expected %q, got %q", "Teresa", s)
			}
		})
	}

	// Multibyte UTF-8 (accents) must also survive intact.
	t.Run("multibyte_utf8", func(t *testing.T) {
		result, err := migrator.applyTransform([]byte("José"), "", int64(1))
		testutil.AssertNoError(t, err)
		if result != "José" {
			t.Errorf("multibyte: expected %q, got %q (%T)", "José", result, result)
		}
	})
}

// TestRetryRowsIndividually proves the per-row fallback that the silent-loss fix
// activates: when a bulk COPY fails, each row is retried individually and every
// one is accounted for — imported on success, logged + skipped on failure — so
// imported+skipped always equals the batch size. (Regression for the 56k-row
// silent loss, where per-row failures returned nil and vanished.)
func TestRetryRowsIndividually(t *testing.T) {
	t.Parallel()

	mysqlClient := testutil.NewMockMySQLClient()
	pgClient := testutil.NewMockPostgresClient()
	state := createTestState(10000, "test-retry-rows")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)
	defer migrator.errorLogger.Close()

	// Single-row InsertBatch fails for even ids, succeeds for odd ids.
	pgClient.InsertBatchFunc = func(rows [][]interface{}) (int, error) {
		id := rows[0][0].(int64)
		if id%2 == 0 {
			return 0, fmt.Errorf("simulated constraint violation for id %d", id)
		}
		return 1, nil
	}

	insertRows := [][]interface{}{{int64(1)}, {int64(2)}, {int64(3)}, {int64(4)}}
	rowIDs := []int64{1, 2, 3, 4}

	imported, skipped, rerr := migrator.retryRowsIndividually(context.Background(), insertRows, rowIDs)
	testutil.AssertNoError(t, rerr)
	if imported != 2 || skipped != 2 {
		t.Fatalf("expected imported=2 skipped=2, got imported=%d skipped=%d", imported, skipped)
	}
	if got := migrator.errorLogger.Count(); got != 2 {
		t.Errorf("expected 2 error-log entries (one per skipped row), got %d", got)
	}
}

// TestRetryRowsIndividuallyStopsOnCancel verifies the fallback bails out when the
// context is cancelled instead of flooding the error log with one entry per
// remaining row.
func TestRetryRowsIndividuallyStopsOnCancel(t *testing.T) {
	t.Parallel()

	mysqlClient := testutil.NewMockMySQLClient()
	pgClient := testutil.NewMockPostgresClient()
	state := createTestState(10000, "test-retry-cancel")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)
	defer migrator.errorLogger.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	insertRows := [][]interface{}{{int64(1)}, {int64(2)}, {int64(3)}}
	rowIDs := []int64{1, 2, 3}

	imported, skipped, rerr := migrator.retryRowsIndividually(ctx, insertRows, rowIDs)
	if rerr == nil {
		t.Fatal("expected context error, got nil")
	}
	if imported != 0 || skipped != 0 {
		t.Errorf("expected no rows processed after immediate cancel, got imported=%d skipped=%d", imported, skipped)
	}
	if got := migrator.errorLogger.Count(); got != 0 {
		t.Errorf("expected 0 error-log entries after immediate cancel, got %d", got)
	}
}

func TestApplyTransformUnknown(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-session-unknown")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	unknownTransforms := []string{
		"bogus_transform",
		"json_to_text",
		"UNKNOWN",
		"text_to_integer",
	}

	for _, transform := range unknownTransforms {
		t.Run(transform, func(t *testing.T) {
			_, err := migrator.applyTransform("any value", transform, int64(1))
			if err == nil {
				t.Fatalf("Expected error for unknown transform %q, got nil", transform)
			}
		})
	}
}

func TestGetDefaultValueForUnmappedColumn(t *testing.T) {
	t.Parallel()

	mysqlClient, pgClient := setupTestClients()
	state := createTestState(10000, "test-default-values")
	cfg := createTestMigrationConfig()

	migrator, err := NewMigrator(mysqlClient, pgClient, cfg, state)
	testutil.AssertNoError(t, err)

	// Inject specific target column metadata
	migrator.targetColumns = map[string]db.ColumnInfo{
		"nullable_col":    {Name: "nullable_col", DataType: "varchar", IsNullable: true},
		"has_default_col": {Name: "has_default_col", DataType: "integer", IsNullable: false, HasDefault: true},
		"jsonb_col":       {Name: "jsonb_col", DataType: "jsonb", IsNullable: false},
		"text_col":        {Name: "text_col", DataType: "text", IsNullable: false},
		"varchar_col":     {Name: "varchar_col", DataType: "varchar", IsNullable: false},
		"charvar_col":     {Name: "charvar_col", DataType: "character varying", IsNullable: false},
		"int_col":         {Name: "int_col", DataType: "integer", IsNullable: false},
		"bigint_col":      {Name: "bigint_col", DataType: "bigint", IsNullable: false},
		"bool_col":        {Name: "bool_col", DataType: "boolean", IsNullable: false},
		"unknown_col":     {Name: "unknown_col", DataType: "bytea", IsNullable: false},
	}

	tests := []struct {
		colName  string
		expected interface{}
	}{
		{"nullable_col", nil},    // nullable → nil
		{"has_default_col", nil}, // has DB default → nil (PG uses the default)
		{"jsonb_col", "{}"},      // NOT NULL jsonb → "{}"
		{"text_col", ""},         // NOT NULL text → ""
		{"varchar_col", ""},      // NOT NULL varchar → ""
		{"charvar_col", ""},      // NOT NULL character varying → ""
		{"int_col", 0},           // NOT NULL integer → 0
		{"bigint_col", 0},        // NOT NULL bigint → 0
		{"bool_col", false},      // NOT NULL boolean → false
		{"unknown_col", nil},     // unknown type → nil
		{"nonexistent_col", nil}, // column not in map → nil
	}

	for _, tt := range tests {
		t.Run(tt.colName, func(t *testing.T) {
			result := migrator.getDefaultValueForUnmappedColumn(tt.colName)
			if result != tt.expected {
				t.Errorf("getDefaultValueForUnmappedColumn(%q) = %v (%T), want %v (%T)",
					tt.colName, result, result, tt.expected, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		slice    []string
		s        string
		expected bool
	}{
		{"found at start", []string{"a", "b", "c"}, "a", true},
		{"found in middle", []string{"a", "b", "c"}, "b", true},
		{"found at end", []string{"a", "b", "c"}, "c", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"case sensitive not found", []string{"hello"}, "Hello", false},
		{"exact match", []string{"foo"}, "foo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.slice, tt.s)
			if result != tt.expected {
				t.Errorf("containsString(%v, %q) = %v, want %v", tt.slice, tt.s, result, tt.expected)
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
