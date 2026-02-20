package testutil

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RobertoGongora/pgpipe/internal/db"
)

// MockMySQLClient provides a configurable mock for MySQLClientInterface.
// Use NewMockMySQLClient() for sensible defaults or configure fields directly.
type MockMySQLClient struct {
	// Responses - configure these to control mock behavior
	PingErr     error
	Tables      []db.TableInfo
	TablesErr   error
	Columns     []db.ColumnInfo
	ColumnsErr  error
	RowCount    int64
	RowCountErr error
	MinID       int64
	MaxID       int64
	MinMaxErr   error
	BatchRows   *sql.Rows
	BatchErr    error

	// Call tracking - check these to verify interactions
	Calls MockMySQLCalls
}

// MockMySQLCalls tracks method invocations
type MockMySQLCalls struct {
	Ping             int
	Close            int
	GetTables        int
	GetColumns       int
	GetPrimaryKey    int
	GetTableRowCount int
	GetMinMaxID      int
	FetchBatch       int
}

// NewMockMySQLClient creates a MockMySQLClient with sensible defaults
func NewMockMySQLClient() *MockMySQLClient {
	return &MockMySQLClient{
		Tables:   GenerateMockTableInfo(5),
		Columns:  GenerateMockColumnInfo(4, true),
		RowCount: 10000,
		MinID:    1,
		MaxID:    10000,
	}
}

// Ping implements db.MySQLClientInterface
func (m *MockMySQLClient) Ping(ctx context.Context) error {
	m.Calls.Ping++
	return m.PingErr
}

// Close implements db.MySQLClientInterface
func (m *MockMySQLClient) Close() error {
	m.Calls.Close++
	return nil
}

// GetTables implements db.MySQLClientInterface
func (m *MockMySQLClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	m.Calls.GetTables++
	if m.TablesErr != nil {
		return nil, m.TablesErr
	}
	return m.Tables, nil
}

// GetColumns implements db.MySQLClientInterface
func (m *MockMySQLClient) GetColumns(ctx context.Context, tableName string) ([]db.ColumnInfo, error) {
	m.Calls.GetColumns++
	if m.ColumnsErr != nil {
		return nil, m.ColumnsErr
	}
	return m.Columns, nil
}

// GetPrimaryKey implements db.MySQLClientInterface
func (m *MockMySQLClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	m.Calls.GetPrimaryKey++
	for _, col := range m.Columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}

// GetTableRowCount implements db.MySQLClientInterface
func (m *MockMySQLClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	m.Calls.GetTableRowCount++
	if m.RowCountErr != nil {
		return 0, m.RowCountErr
	}
	return m.RowCount, nil
}

// GetMinMaxID implements db.MySQLClientInterface
func (m *MockMySQLClient) GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error) {
	m.Calls.GetMinMaxID++
	if m.MinMaxErr != nil {
		return 0, 0, m.MinMaxErr
	}
	return m.MinID, m.MaxID, nil
}

// FetchBatch implements db.MySQLClientInterface
// Note: Returns nil by default since sql.Rows is hard to mock.
// Tests that need FetchBatch behavior should set BatchRows or BatchErr.
func (m *MockMySQLClient) FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error) {
	m.Calls.FetchBatch++
	if m.BatchErr != nil {
		return nil, m.BatchErr
	}
	return m.BatchRows, nil
}

// Reset clears all call tracking counters
func (m *MockMySQLClient) Reset() {
	m.Calls = MockMySQLCalls{}
}

// MockPostgresClient provides a configurable mock for PostgresClientInterface.
// Use NewMockPostgresClient() for sensible defaults or configure fields directly.
type MockPostgresClient struct {
	// Responses - configure these to control mock behavior
	PingErr       error
	Tables        []db.TableInfo
	TablesErr     error
	Columns       []db.ColumnInfo
	ColumnsErr    error
	RowCount      int64
	RowCountErr   error
	InsertedCount int
	InsertErr     error

	// Call tracking - check these to verify interactions
	Calls            MockPostgresCalls
	LastInsertedRows [][]interface{} // Stores the last rows passed to InsertBatch
}

// MockPostgresCalls tracks method invocations
type MockPostgresCalls struct {
	Ping             int
	Close            int
	GetTables        int
	GetColumns       int
	GetPrimaryKey    int
	GetTableRowCount int
	InsertBatch      int
}

// NewMockPostgresClient creates a MockPostgresClient with sensible defaults
func NewMockPostgresClient() *MockPostgresClient {
	return &MockPostgresClient{
		Tables:        GenerateMockTableInfo(5),
		Columns:       GenerateMockColumnInfo(4, true),
		InsertedCount: 1000,
	}
}

// Ping implements db.PostgresClientInterface
func (m *MockPostgresClient) Ping(ctx context.Context) error {
	m.Calls.Ping++
	return m.PingErr
}

// Close implements db.PostgresClientInterface
func (m *MockPostgresClient) Close() {
	m.Calls.Close++
}

// GetTables implements db.PostgresClientInterface
func (m *MockPostgresClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	m.Calls.GetTables++
	if m.TablesErr != nil {
		return nil, m.TablesErr
	}
	return m.Tables, nil
}

// GetColumns implements db.PostgresClientInterface
func (m *MockPostgresClient) GetColumns(ctx context.Context, tableName string) ([]db.ColumnInfo, error) {
	m.Calls.GetColumns++
	if m.ColumnsErr != nil {
		return nil, m.ColumnsErr
	}
	return m.Columns, nil
}

// GetPrimaryKey implements db.PostgresClientInterface
func (m *MockPostgresClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	m.Calls.GetPrimaryKey++
	for _, col := range m.Columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}
	return "", fmt.Errorf("no primary key found")
}

// GetTableRowCount implements db.PostgresClientInterface
func (m *MockPostgresClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	m.Calls.GetTableRowCount++
	if m.RowCountErr != nil {
		return 0, m.RowCountErr
	}
	return m.RowCount, nil
}

// InsertBatch implements db.PostgresClientInterface
func (m *MockPostgresClient) InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error) {
	m.Calls.InsertBatch++
	m.LastInsertedRows = rows
	if m.InsertErr != nil {
		return 0, m.InsertErr
	}
	if m.InsertedCount > 0 {
		return m.InsertedCount, nil
	}
	return len(rows), nil
}

// Reset clears all call tracking counters and stored data
func (m *MockPostgresClient) Reset() {
	m.Calls = MockPostgresCalls{}
	m.LastInsertedRows = nil
}
