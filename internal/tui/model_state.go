package tui

import (
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
)

// ConnectionState holds database connection status
type ConnectionState struct {
	MySQLConnected bool
	PGConnected    bool
	MySQLError     string
	PGError        string
}

// DataCache holds data fetched from databases
type DataCache struct {
	MySQLTables  []db.TableInfo
	PGTables     []db.TableInfo
	MySQLColumns []db.ColumnInfo
	PGColumns    []db.ColumnInfo
}

// SelectionState holds user selections and navigation state
type SelectionState struct {
	SourceTable     string
	SourceTableIdx  int
	TargetTable     string
	TargetTableIdx  int
	SelectedColumns map[string]bool
	ColumnMappings  []config.ColumnMapping
	ColumnCursor    int
	TableCursor     int
	MappingCursor   int
}

// SettingsState holds migration settings and input editing state
type SettingsState struct {
	BatchSize         int
	RunMode           migration.RunMode
	BatchLimit        int
	SettingsCursor    int
	EditingBatchSize  bool
	EditingBatchLimit bool
	InputBuffer       string
}

// MigrationState holds active migration state
type MigrationState struct {
	Migrator *migration.Migrator
	Stats    migration.MigrationStats
	Done     bool
}

// UIState holds ephemeral UI state (modals, search, etc.)
type UIState struct {
	// Resume handling
	HasExistingState bool
	ResumeChoice     int // 0=resume, 1=new

	// Mapping editor
	EditingMapping   bool
	EditTargetCursor int
	AvailableTargets []db.ColumnInfo

	// Column search/filter
	SearchMode      bool
	SearchQuery     string
	FilteredColumns []db.ColumnInfo

	// Table search/filter
	TableSearchMode  bool
	TableSearchQuery string
	FilteredTables   []db.TableInfo
}
