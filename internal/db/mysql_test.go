package db

import (
	"context"
	"errors"
	"testing"
)

func TestMockMySQLClient(t *testing.T) {
	t.Parallel()

	// Create test data
	tables := []TableInfo{
		{Name: "table_1", RowCount: 1000},
		{Name: "table_2", RowCount: 2000},
		{Name: "table_3", RowCount: 3000},
	}

	columns := []ColumnInfo{
		{Name: "id", DataType: "bigint", IsPrimaryKey: true},
		{Name: "name", DataType: "varchar"},
		{Name: "email", DataType: "varchar"},
		{Name: "created_at", DataType: "timestamp"},
		{Name: "updated_at", DataType: "timestamp"},
	}

	mock := &MockMySQLClient{
		Tables:   tables,
		Columns:  columns,
		RowCount: 5000,
		MinID:    1,
		MaxID:    5000,
	}

	ctx := context.Background()

	// Test Ping
	err := mock.Ping(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if mock.PingCalled != 1 {
		t.Errorf("Expected PingCalled=1, got %d", mock.PingCalled)
	}

	// Test GetTables
	resultTables, err := mock.GetTables(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(resultTables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(resultTables))
	}
	if mock.GetTablesCalled != 1 {
		t.Errorf("Expected GetTablesCalled=1, got %d", mock.GetTablesCalled)
	}

	// Test GetColumns
	resultColumns, err := mock.GetColumns(ctx, "test_table")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(resultColumns) != 5 {
		t.Errorf("Expected 5 columns, got %d", len(resultColumns))
	}
	if mock.GetColumnsCalled != 1 {
		t.Errorf("Expected GetColumnsCalled=1, got %d", mock.GetColumnsCalled)
	}

	// Verify first column is PK
	if !resultColumns[0].IsPrimaryKey {
		t.Error("Expected first column to be PK")
	}

	// Test GetPrimaryKey
	pk, err := mock.GetPrimaryKey(ctx, "test_table")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if pk != "id" {
		t.Errorf("Expected pk='id', got '%s'", pk)
	}

	// Test GetTableRowCount
	count, err := mock.GetTableRowCount(ctx, "test_table")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if count != 5000 {
		t.Errorf("Expected count=5000, got %d", count)
	}

	// Test GetMinMaxID
	minID, maxID, err := mock.GetMinMaxID(ctx, "test_table", "id")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if minID != 1 {
		t.Errorf("Expected minID=1, got %d", minID)
	}
	if maxID != 5000 {
		t.Errorf("Expected maxID=5000, got %d", maxID)
	}
}

func TestMockMySQLClientErrors(t *testing.T) {
	t.Parallel()

	testErr := errors.New("mock error")
	mock := &MockMySQLClient{
		TablesError: testErr,
	}

	ctx := context.Background()

	// Test error handling
	_, err := mock.GetTables(ctx)
	if err == nil {
		t.Error("Expected an error, got nil")
	}
}

func TestMockMySQLClientReset(t *testing.T) {
	t.Parallel()

	mock := &MockMySQLClient{}
	ctx := context.Background()

	// Make some calls
	mock.Ping(ctx)
	mock.GetTables(ctx)
	mock.GetColumns(ctx, "test")

	// Verify calls were tracked
	if mock.PingCalled != 1 {
		t.Errorf("Expected PingCalled=1, got %d", mock.PingCalled)
	}
	if mock.GetTablesCalled != 1 {
		t.Errorf("Expected GetTablesCalled=1, got %d", mock.GetTablesCalled)
	}
	if mock.GetColumnsCalled != 1 {
		t.Errorf("Expected GetColumnsCalled=1, got %d", mock.GetColumnsCalled)
	}

	// Reset
	mock.Reset()

	// Verify counts are cleared
	if mock.PingCalled != 0 {
		t.Errorf("Expected PingCalled=0 after reset, got %d", mock.PingCalled)
	}
	if mock.GetTablesCalled != 0 {
		t.Errorf("Expected GetTablesCalled=0 after reset, got %d", mock.GetTablesCalled)
	}
	if mock.GetColumnsCalled != 0 {
		t.Errorf("Expected GetColumnsCalled=0 after reset, got %d", mock.GetColumnsCalled)
	}
}

func TestMockPostgresClient(t *testing.T) {
	t.Parallel()

	tables := []TableInfo{
		{Name: "public.users", RowCount: 100},
		{Name: "public.orders", RowCount: 500},
	}

	columns := []ColumnInfo{
		{Name: "id", DataType: "bigint", IsPrimaryKey: true},
		{Name: "name", DataType: "text"},
	}

	mock := &MockPostgresClient{
		Tables:        tables,
		Columns:       columns,
		InsertedCount: 50,
	}

	ctx := context.Background()

	// Test Ping
	err := mock.Ping(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test GetTables
	resultTables, err := mock.GetTables(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if len(resultTables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(resultTables))
	}

	// Test InsertBatch
	testRows := [][]interface{}{
		{1, "Alice"},
		{2, "Bob"},
		{3, "Charlie"},
	}

	inserted, err := mock.InsertBatch(ctx, "users", []string{"id", "name"}, testRows)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if inserted != 50 {
		t.Errorf("Expected inserted=50, got %d", inserted)
	}
	if mock.InsertBatchCalled != 1 {
		t.Errorf("Expected InsertBatchCalled=1, got %d", mock.InsertBatchCalled)
	}
	if len(mock.LastInsertedRows) != 3 {
		t.Errorf("Expected 3 rows tracked, got %d", len(mock.LastInsertedRows))
	}
}
