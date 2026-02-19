package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
)

// Screen represents the current screen being displayed
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenConnections
	ScreenSourceTable
	ScreenSourceColumns
	ScreenTargetTable
	ScreenMapping
	ScreenSettings
	ScreenRunning
	ScreenSummary
)

// Run starts the TUI application
func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Model is the main application model
type Model struct {
	// Core application state
	screen      Screen
	config      *config.Config
	state       *migration.State
	mysqlClient db.MySQLClientInterface
	pgClient    db.PostgresClientInterface
	err         error
	width       int
	height      int
	quitting    bool

	// Grouped state
	conn      ConnectionState
	data      DataCache
	selection SelectionState
	settings  SettingsState
	migration MigrationState
	ui        UIState
}

// NewModel creates a new application model
func NewModel() Model {
	// Try to load existing config first
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	// Check for existing state
	existingState, _ := migration.LoadState()

	// Pre-populate fields from saved config
	selectedColumns := make(map[string]bool)
	for _, col := range cfg.Migration.Source.Columns {
		selectedColumns[col] = true
	}

	// Use saved batch settings or defaults
	batchSize := 5000
	if cfg.Migration.Settings.BatchSize > 0 {
		batchSize = cfg.Migration.Settings.BatchSize
	}

	return Model{
		screen: ScreenWelcome,
		config: cfg,
		state:  existingState,
		conn:   ConnectionState{},
		data:   DataCache{},
		selection: SelectionState{
			SelectedColumns: selectedColumns,
			SourceTable:     cfg.Migration.Source.Table,
			TargetTable:     cfg.Migration.Target.Table,
			ColumnMappings:  cfg.Migration.Mapping,
		},
		settings: SettingsState{
			BatchSize:  batchSize,
			RunMode:    migration.RunModeBatches,
			BatchLimit: 100,
		},
		migration: MigrationState{},
		ui: UIState{
			HasExistingState: existingState != nil,
		},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		m.testConnections,
	)
}

// testConnections tests MySQL and PostgreSQL connections
func (m Model) testConnections() tea.Msg {
	return ConnectionTestMsg{
		mysqlErr: nil,
		pgErr:    nil,
	}
}

// Update handles messages and user input
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Global quit handling
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// Screen-specific key handling
		return m.handleKeyPress(msg)

	case ConnectionTestMsg:
		return m.handleConnectionTest(msg)

	case MySQLTablesMsg:
		m.data.MySQLTables = msg.tables
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case PGTablesMsg:
		m.data.PGTables = msg.tables
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case MySQLColumnsMsg:
		m.data.MySQLColumns = msg.columns
		if msg.err != nil {
			m.err = msg.err
		}
		// Auto-select all columns by default (but only if not resuming)
		if len(m.selection.SelectedColumns) == 0 {
			for _, col := range msg.columns {
				if !col.IsPrimaryKey {
					m.selection.SelectedColumns[col.Name] = true
				}
			}
		}
		// If we're on Connections screen and have both clients, we're resuming
		// Wait for PG columns then auto-advance to Settings
		if m.screen == ScreenConnections && m.data.PGColumns != nil {
			m.screen = ScreenSettings
		}
		return m, nil

	case PGColumnsMsg:
		m.data.PGColumns = msg.columns
		if msg.err != nil {
			m.err = msg.err
		}
		// Auto-generate mappings (but only if not resuming with saved mappings)
		if len(m.selection.ColumnMappings) == 0 {
			m.generateAutoMappings()
		}
		// If we're on Connections screen and have both clients, we're resuming
		// Auto-advance to Settings
		if m.screen == ScreenConnections && m.data.MySQLColumns != nil {
			m.screen = ScreenSettings
		}
		return m, nil

	case MigrationProgressMsg:
		m.migration.Stats = msg.stats
		return m, nil

	case MigrationDoneMsg:
		m.migration.Done = true
		if msg.err != nil {
			m.err = msg.err
		}
		m.screen = ScreenSummary
		return m, nil

	case MigrationInitializingMsg:
		// Update the initialization progress message
		m.err = fmt.Errorf(msg.message) // Reuse err field for progress display
		return m, nil

	case MigrationInitErrorMsg:
		// Initialization failed - show error
		m.err = msg.err
		m.screen = ScreenSummary
		return m, nil

	case MigrationStartedMsg:
		// Store the migrator and state that were created in the command
		m.migration.Migrator = msg.migrator
		m.state = msg.state
		m.err = nil // Clear any initialization progress messages
		// Start TWO commands:
		// 1. Tick loop for UI updates
		// 2. Completion listener that waits on the done channel
		return m, tea.Batch(
			tickAfter(MigrationTickInterval),
			waitForMigrationCompletion(msg.done),
		)

	case TickMsg:
		// Only process ticks when migration is running
		if m.screen == ScreenRunning && m.migration.Migrator != nil {
			// Refresh state from migrator
			m.state = m.migration.Migrator.GetState()

			// Update stats for display
			if m.state != nil {
				errorLogger := m.migration.Migrator.GetErrorLogger()
				m.migration.Stats = migration.MigrationStats{
					BatchesCompleted: m.state.Batches.Completed,
					RowsProcessed:    m.state.Progress.ProcessedRows,
					RowsImported:     m.state.Progress.ImportedRows,
					RowsSkipped:      m.state.Progress.SkippedRows,
					CurrentCursor:    m.state.Progress.LastCursor,
					TotalRows:        m.state.Source.TotalRows,
					ErrorLogPath:     errorLogger.Path(),
				}
			}

			// Check if migration is complete
			if m.state != nil && m.state.IsComplete() {
				m.migration.Done = true
				m.screen = ScreenSummary
				return m, nil
			}

			// Schedule next tick
			return m, tickAfter(MigrationTickInterval)
		}
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles key presses for the current screen
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if we're in mapping editor mode
	if m.ui.EditingMapping {
		return m.handleMappingEditorKeys(msg)
	}

	switch m.screen {
	case ScreenWelcome:
		return m.handleWelcomeKeys(msg)
	case ScreenConnections:
		return m.handleConnectionsKeys(msg)
	case ScreenSourceTable:
		return m.handleSourceTableKeys(msg)
	case ScreenSourceColumns:
		return m.handleSourceColumnsKeys(msg)
	case ScreenTargetTable:
		return m.handleTargetTableKeys(msg)
	case ScreenMapping:
		return m.handleMappingKeys(msg)
	case ScreenSettings:
		return m.handleSettingsKeys(msg)
	case ScreenRunning:
		return m.handleRunningKeys(msg)
	case ScreenSummary:
		return m.handleSummaryKeys(msg)
	}
	return m, nil
}

// generateAutoMappings creates column mappings based on name matching
func (m *Model) generateAutoMappings() {
	m.selection.ColumnMappings = nil

	// Build a map of target columns
	targetCols := make(map[string]db.ColumnInfo)
	for _, col := range m.data.PGColumns {
		targetCols[col.Name] = col
	}

	// For each selected source column, try to find a match
	for _, srcCol := range m.data.MySQLColumns {
		if !m.selection.SelectedColumns[srcCol.Name] {
			continue
		}

		mapping := config.ColumnMapping{
			Source: srcCol.Name,
		}

		// Try exact name match
		if tgtCol, ok := targetCols[srcCol.Name]; ok {
			mapping.Target = tgtCol.Name

			// Detect if transform is needed
			if (isTextType(srcCol.DataType) || isJSONSourceType(srcCol.DataType)) && isJSONType(tgtCol.DataType) {
				mapping.Transform = "text_to_jsonb"
			} else if isIntType(srcCol.DataType) && isBoolType(tgtCol.DataType) {
				mapping.Transform = "int_to_bool"
			} else if isTextType(srcCol.DataType) && isUUIDType(tgtCol.DataType) {
				mapping.Transform = "string_to_uuid"
			}
		}

		m.selection.ColumnMappings = append(m.selection.ColumnMappings, mapping)
	}
}

// View renders the current screen
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Check if we're in mapping editor mode
	if m.ui.EditingMapping {
		return m.viewMappingEditor()
	}

	switch m.screen {
	case ScreenWelcome:
		return m.viewWelcome()
	case ScreenConnections:
		return m.viewConnections()
	case ScreenSourceTable:
		return m.viewSourceTable()
	case ScreenSourceColumns:
		return m.viewSourceColumns()
	case ScreenTargetTable:
		return m.viewTargetTable()
	case ScreenMapping:
		return m.viewMapping()
	case ScreenSettings:
		return m.viewSettings()
	case ScreenRunning:
		return m.viewRunning()
	case ScreenSummary:
		return m.viewSummary()
	default:
		return "Unknown screen"
	}
}
