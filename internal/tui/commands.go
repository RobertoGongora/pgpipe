package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/db"
	"github.com/RobertoGongora/pgpipe/internal/migration"
)

// connectDatabases creates connections to MySQL and PostgreSQL
func (m Model) connectDatabases() tea.Msg {
	var mysqlErr, pgErr error

	// Connect to MySQL
	mysqlClient, err := db.NewMySQLClient(&m.config.MySQL)
	if err != nil {
		mysqlErr = err
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectionTimeout)
		defer cancel()
		if err := mysqlClient.Ping(ctx); err != nil {
			mysqlErr = err
			mysqlClient.Close()
		}
	}

	// Connect to PostgreSQL
	pgClient, err := db.NewPostgresClient(&m.config.PostgreSQL)
	if err != nil {
		pgErr = err
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectionTimeout)
		defer cancel()
		if err := pgClient.Ping(ctx); err != nil {
			pgErr = err
			pgClient.Close()
		}
	}

	return ConnectionTestMsg{
		mysqlErr: mysqlErr,
		pgErr:    pgErr,
	}
}

// handleConnectionTest processes the result of connection testing
func (m Model) handleConnectionTest(msg ConnectionTestMsg) (Model, tea.Cmd) {
	if msg.mysqlErr != nil {
		m.conn.MySQLError = msg.mysqlErr.Error()
		m.conn.MySQLConnected = false
	} else {
		m.conn.MySQLConnected = true
		m.conn.MySQLError = ""
		// Store client
		client, _ := db.NewMySQLClient(&m.config.MySQL)
		m.mysqlClient = client
	}

	if msg.pgErr != nil {
		m.conn.PGError = msg.pgErr.Error()
		m.conn.PGConnected = false
	} else {
		m.conn.PGConnected = true
		m.conn.PGError = ""
		// Store client
		client, _ := db.NewPostgresClient(&m.config.PostgreSQL)
		m.pgClient = client
	}

	// Load tables if both connected
	if m.conn.MySQLConnected && m.conn.PGConnected {
		// Check if we're resuming with pre-configured tables
		if m.selection.SourceTable != "" && m.selection.TargetTable != "" && len(m.selection.ColumnMappings) > 0 {
			// We're resuming - load the columns for the saved tables
			return m, tea.Batch(
				m.loadMySQLColumns,
				m.loadPGColumns,
			)
		}
		// Normal flow - load table lists
		return m, tea.Batch(m.loadMySQLTables, m.loadPGTables)
	}

	return m, nil
}

// loadMySQLTables fetches the list of tables from MySQL
func (m Model) loadMySQLTables() tea.Msg {
	if m.mysqlClient == nil {
		return MySQLTablesMsg{err: fmt.Errorf("not connected")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	tables, err := m.mysqlClient.GetTables(ctx)
	return MySQLTablesMsg{tables: tables, err: err}
}

// loadPGTables fetches the list of tables from PostgreSQL
func (m Model) loadPGTables() tea.Msg {
	if m.pgClient == nil {
		return PGTablesMsg{err: fmt.Errorf("not connected")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	tables, err := m.pgClient.GetTables(ctx)
	return PGTablesMsg{tables: tables, err: err}
}

// loadMySQLColumns fetches columns for the selected MySQL table
func (m Model) loadMySQLColumns() tea.Msg {
	if m.mysqlClient == nil || m.selection.SourceTable == "" {
		return MySQLColumnsMsg{err: fmt.Errorf("not ready")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	columns, err := m.mysqlClient.GetColumns(ctx, m.selection.SourceTable)
	return MySQLColumnsMsg{columns: columns, err: err}
}

// loadPGColumns fetches columns for the selected PostgreSQL table
func (m Model) loadPGColumns() tea.Msg {
	if m.pgClient == nil || m.selection.TargetTable == "" {
		return PGColumnsMsg{err: fmt.Errorf("not ready")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	columns, err := m.pgClient.GetColumns(ctx, m.selection.TargetTable)
	return PGColumnsMsg{columns: columns, err: err}
}

// startMigration initializes and starts the migration process
func (m Model) startMigration() tea.Cmd {
	// Capture all values we need from the model
	// This is necessary because the command runs asynchronously
	mysqlClient := m.mysqlClient
	pgClient := m.pgClient
	sourceTable := m.selection.SourceTable
	targetTable := m.selection.TargetTable
	columnMappings := m.selection.ColumnMappings
	batchSize := m.settings.BatchSize
	runMode := m.settings.RunMode
	batchLimit := m.settings.BatchLimit
	mysqlColumns := m.data.MySQLColumns
	existingState := m.state
	hasExistingState := m.ui.HasExistingState

	// Return a command that does ALL the work asynchronously
	// This function will run in a goroutine by Bubble Tea
	return func() tea.Msg {
		// Build source columns list
		var sourceColumns []string
		var targetColumns []string
		for _, mapping := range columnMappings {
			if mapping.Target != "" {
				sourceColumns = append(sourceColumns, mapping.Source)
				targetColumns = append(targetColumns, mapping.Target)
			}
		}

		// Get primary key
		var pkColumn string
		for _, col := range mysqlColumns {
			if col.IsPrimaryKey {
				pkColumn = col.Name
				break
			}
		}

		// Create or load state
		var state *migration.State
		if existingState != nil && hasExistingState {
			// Resuming - fast path, no DB calls needed
			state = existingState
		} else {
			// New migration - need to initialize
			newCfg := &config.Config{
				Migration: config.MigrationConfig{
					Source: config.SourceConfig{
						Table:      sourceTable,
						PrimaryKey: pkColumn,
						Columns:    sourceColumns,
					},
					Target: config.TargetConfig{
						Table: targetTable,
					},
					Mapping: columnMappings,
					Settings: config.SettingsConfig{
						BatchSize: batchSize,
					},
				},
			}
			state = migration.NewState(newCfg.Hash())

			// Initialize source info - THESE ARE THE SLOW OPERATIONS
			// But now they run in a goroutine so UI stays responsive!
			ctx := context.Background()

			// Progress indicator 1: Counting rows
			totalRows, err := mysqlClient.GetTableRowCount(ctx, sourceTable)
			if err != nil {
				return MigrationInitErrorMsg{
					err: fmt.Errorf("failed to count rows: %w", err),
				}
			}

			// Progress indicator 2: Getting ID range
			minID, maxID, err := mysqlClient.GetMinMaxID(ctx, sourceTable, pkColumn)
			if err != nil {
				return MigrationInitErrorMsg{
					err: fmt.Errorf("failed to get ID range: %w", err),
				}
			}

			state.Source = migration.SourceState{
				Table:      sourceTable,
				TotalRows:  totalRows,
				PrimaryKey: pkColumn,
				MinID:      minID,
				MaxID:      maxID,
			}
			state.Batches.Size = batchSize
		}

		// Create migrator
		migCfg := migration.MigrationConfig{
			SourceTable:   sourceTable,
			TargetTable:   targetTable,
			SourcePK:      pkColumn,
			SourceColumns: sourceColumns,
			TargetColumns: targetColumns,
			Mapping:       columnMappings,
			BatchSize:     batchSize,
			Mode:          runMode,
			BatchLimit:    batchLimit,
		}

		migrator, err := migration.NewMigrator(mysqlClient, pgClient, migCfg, state)
		if err != nil {
			return MigrationInitErrorMsg{
				err: fmt.Errorf("failed to create migrator: %w", err),
			}
		}

		// Create done channel for completion notification
		done := make(chan error, 1)

		// Run migration in goroutine
		go func() {
			migration.LogDebug("[GOROUTINE] Migration goroutine started")
			ctx := context.Background()
			err := migrator.Run(ctx)
			if err != nil {
				migration.LogDebug("[GOROUTINE] Migration completed with ERROR: %v", err)
			} else {
				migration.LogDebug("[GOROUTINE] Migration completed successfully")
			}
			// Send completion signal with any error
			done <- err
			migration.LogDebug("[GOROUTINE] Sent completion signal to channel")
		}()

		// Return success - migrator is ready!
		return MigrationStartedMsg{
			migrator: migrator,
			state:    state,
			done:     done,
		}
	}
}

// tickAfter returns a command that sends TickMsg after duration
func tickAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// waitForMigrationCompletion returns a command that waits for the migration to complete
func waitForMigrationCompletion(done chan error) tea.Cmd {
	return func() tea.Msg {
		err := <-done // Block until migration completes
		return MigrationDoneMsg{err: err}
	}
}
