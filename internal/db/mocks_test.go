package db

import (
	"context"
	"database/sql"
	"fmt"
)

// MockMySQLClient is a mock implementation of MySQLClientInterface
type MockMySQLClient struct {
	// Configurable responses
	PingError         error
	Tables            []TableInfo
	TablesError       error
	Columns           []ColumnInfo
	ColumnsError      error
	RowCount          int64
	RowCountError     error
	MinID             int64
	MaxID             int64
	MinMaxError       error
	BatchData         [][][]interface{} // Multiple batches of data
	BatchError        error
	CurrentBatchIndex int

	// Call tracking
	PingCalled             int
	CloseCalled            int
	GetTablesCalled        int
	GetColumnsCalled       int
	GetPrimaryKeyCalled    int
	GetTableRowCountCalled int
	GetMinMaxIDCalled      int
	FetchBatchCalled       int
}

// Ping implements MySQLClientInterface
func (m *MockMySQLClient) Ping(ctx context.Context) error {
	m.PingCalled++
	return m.PingError
}

// Close implements MySQLClientInterface
func (m *MockMySQLClient) Close() error {
	m.CloseCalled++
	return nil
}

// GetTables implements MySQLClientInterface
func (m *MockMySQLClient) GetTables(ctx context.Context) ([]TableInfo, error) {
	m.GetTablesCalled++
	if m.TablesError != nil {
		return nil, m.TablesError
	}
	return m.Tables, nil
}

// GetColumns implements MySQLClientInterface
func (m *MockMySQLClient) GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	m.GetColumnsCalled++
	if m.ColumnsError != nil {
		return nil, m.ColumnsError
	}
	return m.Columns, nil
}

// GetPrimaryKey implements MySQLClientInterface
func (m *MockMySQLClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	m.GetPrimaryKeyCalled++
	for _, col := range m.Columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}

// GetTableRowCount implements MySQLClientInterface
func (m *MockMySQLClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	m.GetTableRowCountCalled++
	if m.RowCountError != nil {
		return 0, m.RowCountError
	}
	return m.RowCount, nil
}

// GetMinMaxID implements MySQLClientInterface
func (m *MockMySQLClient) GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error) {
	m.GetMinMaxIDCalled++
	if m.MinMaxError != nil {
		return 0, 0, m.MinMaxError
	}
	return m.MinID, m.MaxID, nil
}

// FetchBatch implements MySQLClientInterface
// Note: This returns a mock sql.Rows which is complex to implement
// For testing purposes, we'll use a simplified approach
func (m *MockMySQLClient) FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error) {
	m.FetchBatchCalled++
	if m.BatchError != nil {
		return nil, m.BatchError
	}
	// Note: Returning nil here as we can't easily create sql.Rows
	// Tests will need to work around this or we'll refactor FetchBatch to return [][]interface{}
	return nil, nil
}

// MockPostgresClient is a mock implementation of PostgresClientInterface
type MockPostgresClient struct {
	// Configurable responses
	PingError     error
	Tables        []TableInfo
	TablesError   error
	Columns       []ColumnInfo
	ColumnsError  error
	RowCount      int64
	RowCountError error
	InsertedCount int
	InsertError   error

	// Call tracking
	PingCalled             int
	CloseCalled            int
	GetTablesCalled        int
	GetColumnsCalled       int
	GetPrimaryKeyCalled    int
	GetTableRowCountCalled int
	InsertBatchCalled      int
	LastInsertedRows       [][]interface{} // Track last inserted data
}

// Ping implements PostgresClientInterface
func (m *MockPostgresClient) Ping(ctx context.Context) error {
	m.PingCalled++
	return m.PingError
}

// Close implements PostgresClientInterface
func (m *MockPostgresClient) Close() {
	m.CloseCalled++
}

// GetTables implements PostgresClientInterface
func (m *MockPostgresClient) GetTables(ctx context.Context) ([]TableInfo, error) {
	m.GetTablesCalled++
	if m.TablesError != nil {
		return nil, m.TablesError
	}
	return m.Tables, nil
}

// GetColumns implements PostgresClientInterface
func (m *MockPostgresClient) GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	m.GetColumnsCalled++
	if m.ColumnsError != nil {
		return nil, m.ColumnsError
	}
	return m.Columns, nil
}

// GetPrimaryKey implements PostgresClientInterface
func (m *MockPostgresClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	m.GetPrimaryKeyCalled++
	for _, col := range m.Columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}

// GetTableRowCount implements PostgresClientInterface
func (m *MockPostgresClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	m.GetTableRowCountCalled++
	if m.RowCountError != nil {
		return 0, m.RowCountError
	}
	return m.RowCount, nil
}

// InsertBatch implements PostgresClientInterface
func (m *MockPostgresClient) InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error) {
	m.InsertBatchCalled++
	m.LastInsertedRows = rows
	if m.InsertError != nil {
		return 0, m.InsertError
	}
	if m.InsertedCount > 0 {
		return m.InsertedCount, nil
	}
	return len(rows), nil
}

// Reset clears call tracking for test isolation
func (m *MockMySQLClient) Reset() {
	m.PingCalled = 0
	m.CloseCalled = 0
	m.GetTablesCalled = 0
	m.GetColumnsCalled = 0
	m.GetPrimaryKeyCalled = 0
	m.GetTableRowCountCalled = 0
	m.GetMinMaxIDCalled = 0
	m.FetchBatchCalled = 0
	m.CurrentBatchIndex = 0
}

// Reset clears call tracking for test isolation
func (m *MockPostgresClient) Reset() {
	m.PingCalled = 0
	m.CloseCalled = 0
	m.GetTablesCalled = 0
	m.GetColumnsCalled = 0
	m.GetPrimaryKeyCalled = 0
	m.GetTableRowCountCalled = 0
	m.InsertBatchCalled = 0
	m.LastInsertedRows = nil
}
