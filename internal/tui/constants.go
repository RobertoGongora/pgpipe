package tui

import "time"

// Viewport sizes for scrollable lists
const (
	// MaxVisibleTables is the number of tables shown in table selection screens
	MaxVisibleTables = 10

	// MaxVisibleColumns is the number of columns shown in column selection screen
	MaxVisibleColumns = 12

	// MaxVisibleTargets is the number of target columns shown in mapping editor
	MaxVisibleTargets = 10
)

// Input validation limits
const (
	// MinBatchSize is the minimum allowed batch size
	MinBatchSize = 1

	// MaxBatchSize is the maximum allowed batch size (int32 max)
	MaxBatchSize = 2147483647

	// MinBatchLimit is the minimum allowed batch limit
	MinBatchLimit = 1

	// MaxBatchLimit is the maximum allowed batch limit (int32 max)
	MaxBatchLimit = 2147483647
)

// UI refresh rates
const (
	// MigrationTickInterval is how often the UI refreshes during migration
	MigrationTickInterval = 500 * time.Millisecond
)

// Connection timeouts
const (
	// ConnectionTimeout is the timeout for database connection tests
	ConnectionTimeout = 5 * time.Second

	// QueryTimeout is the timeout for database queries
	QueryTimeout = 30 * time.Second
)
