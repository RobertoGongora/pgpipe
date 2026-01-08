package tui

import (
	"database/sql"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
	"github.com/pgpipe/pgpipe/internal/testutil"
)

// createTestModel creates a minimal model for testing
func createTestModel() Model {
	columns := []db.ColumnInfo{
		{Name: "id", DataType: "bigint", IsPrimaryKey: true},
		{Name: "name", DataType: "varchar"},
		{Name: "email", DataType: "varchar"},
	}

	mysqlClient := testutil.NewMockMySQLClient()
	mysqlClient.Columns = columns
	mysqlClient.RowCount = 10000
	mysqlClient.MinID = 1
	mysqlClient.MaxID = 10000
	// Return sql.ErrNoRows to prevent the migration from actually running in tests
	// This is intentional - we're testing the message flow, not the migration logic
	mysqlClient.BatchErr = sql.ErrNoRows

	pgClient := testutil.NewMockPostgresClient()

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
