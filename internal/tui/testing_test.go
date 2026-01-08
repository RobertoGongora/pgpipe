package tui

import (
	"context"
	"database/sql"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
)

// createTestModel creates a minimal model for testing
func createTestModel() Model {
	columns := []db.ColumnInfo{
		{Name: "id", DataType: "bigint", IsPrimaryKey: true},
		{Name: "name", DataType: "varchar"},
		{Name: "email", DataType: "varchar"},
	}

	mysqlClient := &mockMySQLClient{
		rowCount: 10000,
		minID:    1,
		maxID:    10000,
		columns:  columns,
	}

	pgClient := &mockPostgresClient{}

	return Model{
		mysqlClient:  mysqlClient,
		pgClient:     pgClient,
		sourceTable:  "test_table",
		targetTable:  "test_table",
		mysqlColumns: columns,
		batchSize:    1000,
		runMode:      migration.RunModeContinuous,
		columnMappings: []config.ColumnMapping{
			{Source: "name", Target: "name"},
			{Source: "email", Target: "email"},
		},
	}
}

// executeTestCmd executes a Bubble Tea command and returns the message
func executeTestCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// mockMySQLClient is a minimal mock for TUI tests
type mockMySQLClient struct {
	rowCount int64
	minID    int64
	maxID    int64
	columns  []db.ColumnInfo
}

func (m *mockMySQLClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockMySQLClient) Close() error {
	return nil
}

func (m *mockMySQLClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	return nil, nil
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
	return "", nil
}

func (m *mockMySQLClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	return m.rowCount, nil
}

func (m *mockMySQLClient) GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error) {
	return m.minID, m.maxID, nil
}

func (m *mockMySQLClient) FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error) {
	// Return an error to prevent the migration from actually running in tests
	// This is intentional - we're testing the message flow, not the migration logic
	return nil, sql.ErrNoRows
}

// mockPostgresClient is a minimal mock for TUI tests
type mockPostgresClient struct{}

func (m *mockPostgresClient) Ping(ctx context.Context) error {
	return nil
}

func (m *mockPostgresClient) Close() {}

func (m *mockPostgresClient) GetTables(ctx context.Context) ([]db.TableInfo, error) {
	return nil, nil
}

func (m *mockPostgresClient) GetColumns(ctx context.Context, tableName string) ([]db.ColumnInfo, error) {
	return nil, nil
}

func (m *mockPostgresClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	return "", nil
}

func (m *mockPostgresClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	return 0, nil
}

func (m *mockPostgresClient) InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error) {
	return len(rows), nil
}
