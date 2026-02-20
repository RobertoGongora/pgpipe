package tui

import (
	"github.com/RobertoGongora/pgpipe/internal/db"
	"github.com/RobertoGongora/pgpipe/internal/migration"
)

// ConnectionTestMsg is sent after testing database connections
type ConnectionTestMsg struct {
	mysqlErr error
	pgErr    error
}

// MySQLTablesMsg is sent after loading MySQL tables
type MySQLTablesMsg struct {
	tables []db.TableInfo
	err    error
}

// PGTablesMsg is sent after loading PostgreSQL tables
type PGTablesMsg struct {
	tables []db.TableInfo
	err    error
}

// MySQLColumnsMsg is sent after loading MySQL columns
type MySQLColumnsMsg struct {
	columns []db.ColumnInfo
	err     error
}

// PGColumnsMsg is sent after loading PostgreSQL columns
type PGColumnsMsg struct {
	columns []db.ColumnInfo
	err     error
}

// MigrationProgressMsg is sent during migration to update progress display
type MigrationProgressMsg struct {
	stats migration.MigrationStats
}

// MigrationDoneMsg is sent when migration completes (successfully or with error)
type MigrationDoneMsg struct {
	err error
}

// MigrationStartedMsg is sent when migration is initialized and ready to run
type MigrationStartedMsg struct {
	migrator *migration.Migrator
	state    *migration.State
	done     chan error // Channel that signals when migration completes
}

// MigrationInitializingMsg is sent to show initialization progress
type MigrationInitializingMsg struct {
	message string
}

// MigrationInitErrorMsg is sent when migration initialization fails
type MigrationInitErrorMsg struct {
	err error
}

// TickMsg is sent periodically during migration to refresh the UI
type TickMsg struct{}
