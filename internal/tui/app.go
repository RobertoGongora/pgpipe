package tui

import (
	"fmt"
	"strings"
	"time"

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
	screen      Screen
	config      *config.Config
	state       *migration.State
	mysqlClient db.MySQLClientInterface
	pgClient    db.PostgresClientInterface
	err         error
	width       int
	height      int
	quitting    bool

	// Connection status
	mysqlConnected bool
	pgConnected    bool
	mysqlError     string
	pgError        string

	// Table data
	mysqlTables  []db.TableInfo
	pgTables     []db.TableInfo
	mysqlColumns []db.ColumnInfo
	pgColumns    []db.ColumnInfo

	// User selections
	sourceTable     string
	sourceTableIdx  int
	targetTable     string
	targetTableIdx  int
	selectedColumns map[string]bool
	columnCursor    int
	tableCursor     int
	columnMappings  []config.ColumnMapping
	mappingCursor   int

	// Settings
	batchSize         int
	runMode           migration.RunMode
	batchLimit        int
	settingsCursor    int
	editingBatchSize  bool   // Whether we're editing batch size
	editingBatchLimit bool   // Whether we're editing batch limit
	inputBuffer       string // Input buffer for typing numbers

	// Migration
	migrator       *migration.Migrator
	migrationStats migration.MigrationStats
	migrationDone  bool

	// Resume handling
	hasExistingState bool
	resumeChoice     int // 0=resume, 1=new

	// Mapping editor
	editingMapping   bool            // Whether we're in mapping edit mode
	editTargetCursor int             // Cursor position in target column list
	availableTargets []db.ColumnInfo // Available target columns to choose from

	// Search/filter (columns)
	searchMode      bool            // Whether we're in search mode
	searchQuery     string          // Current search query
	filteredColumns []db.ColumnInfo // Filtered column list

	// Search/filter (tables)
	tableSearchMode  bool           // Whether we're in table search mode
	tableSearchQuery string         // Current table search query
	filteredTables   []db.TableInfo // Filtered table list
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
		screen:           ScreenWelcome,
		config:           cfg,
		state:            existingState,
		hasExistingState: existingState != nil,
		selectedColumns:  selectedColumns,
		sourceTable:      cfg.Migration.Source.Table,
		targetTable:      cfg.Migration.Target.Table,
		columnMappings:   cfg.Migration.Mapping,
		batchSize:        batchSize,
		runMode:          migration.RunModeBatches,
		batchLimit:       100,
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

// ConnectionTestMsg is sent after testing connections
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

// MigrationProgressMsg is sent during migration
type MigrationProgressMsg struct {
	stats migration.MigrationStats
}

// MigrationDoneMsg is sent when migration completes
type MigrationDoneMsg struct {
	err error
}

// MigrationStartedMsg is sent when migration is initialized
type MigrationStartedMsg struct {
	migrator *migration.Migrator
	state    *migration.State
	done     chan error // Channel that signals when migration completes
}

// MigrationInitializingMsg is sent to show initialization progress
type MigrationInitializingMsg struct {
	message string
}

// MigrationInitErrorMsg is sent when initialization fails
type MigrationInitErrorMsg struct {
	err error
}

// TickMsg is sent periodically during migration
type TickMsg struct{}

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
		m.mysqlTables = msg.tables
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case PGTablesMsg:
		m.pgTables = msg.tables
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case MySQLColumnsMsg:
		m.mysqlColumns = msg.columns
		if msg.err != nil {
			m.err = msg.err
		}
		// Auto-select all columns by default (but only if not resuming)
		if len(m.selectedColumns) == 0 {
			for _, col := range msg.columns {
				if !col.IsPrimaryKey {
					m.selectedColumns[col.Name] = true
				}
			}
		}
		// If we're on Connections screen and have both clients, we're resuming
		// Wait for PG columns then auto-advance to Settings
		if m.screen == ScreenConnections && m.pgColumns != nil {
			m.screen = ScreenSettings
		}
		return m, nil

	case PGColumnsMsg:
		m.pgColumns = msg.columns
		if msg.err != nil {
			m.err = msg.err
		}
		// Auto-generate mappings (but only if not resuming with saved mappings)
		if len(m.columnMappings) == 0 {
			m.generateAutoMappings()
		}
		// If we're on Connections screen and have both clients, we're resuming
		// Auto-advance to Settings
		if m.screen == ScreenConnections && m.mysqlColumns != nil {
			m.screen = ScreenSettings
		}
		return m, nil

	case MigrationProgressMsg:
		m.migrationStats = msg.stats
		return m, nil

	case MigrationDoneMsg:
		m.migrationDone = true
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
		m.migrator = msg.migrator
		m.state = msg.state
		m.err = nil // Clear any initialization progress messages
		// Start TWO commands:
		// 1. Tick loop for UI updates
		// 2. Completion listener that waits on the done channel
		return m, tea.Batch(
			tickAfter(500*time.Millisecond),
			waitForMigrationCompletion(msg.done),
		)

	case TickMsg:
		// Only process ticks when migration is running
		if m.screen == ScreenRunning && m.migrator != nil {
			// Refresh state from migrator
			m.state = m.migrator.GetState()

			// Update stats for display
			if m.state != nil {
				errorLogger := m.migrator.GetErrorLogger()
				m.migrationStats = migration.MigrationStats{
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
				m.migrationDone = true
				m.screen = ScreenSummary
				return m, nil
			}

			// Schedule next tick
			return m, tickAfter(500 * time.Millisecond)
		}
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles key presses for the current screen
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if we're in mapping editor mode
	if m.editingMapping {
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
	m.columnMappings = nil

	// Build a map of target columns
	targetCols := make(map[string]db.ColumnInfo)
	for _, col := range m.pgColumns {
		targetCols[col.Name] = col
	}

	// For each selected source column, try to find a match
	for _, srcCol := range m.mysqlColumns {
		if !m.selectedColumns[srcCol.Name] {
			continue
		}

		mapping := config.ColumnMapping{
			Source: srcCol.Name,
		}

		// Try exact name match
		if tgtCol, ok := targetCols[srcCol.Name]; ok {
			mapping.Target = tgtCol.Name

			// Detect if transform is needed
			if isTextType(srcCol.DataType) && isJSONType(tgtCol.DataType) {
				mapping.Transform = "text_to_jsonb"
			}
		}

		m.columnMappings = append(m.columnMappings, mapping)
	}
}

func isTextType(dataType string) bool {
	switch dataType {
	case "text", "mediumtext", "longtext", "varchar", "char":
		return true
	}
	return false
}

func isJSONType(dataType string) bool {
	switch dataType {
	case "json", "jsonb":
		return true
	}
	return false
}

// View renders the current screen
func (m Model) View() string {
	if m.quitting {
		return "Goodbye!\n"
	}

	// Check if we're in mapping editor mode
	if m.editingMapping {
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

// Helper to truncate strings
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// Helper to create progress bar
func progressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s] %.1f%%",
		repeatChar('█', filled),
		repeatChar('░', empty),
		percent)
}

func repeatChar(c rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

// filterColumns filters m.mysqlColumns based on search query
func (m *Model) filterColumns() {
	if m.searchQuery == "" {
		m.filteredColumns = nil
		return
	}

	query := strings.ToLower(m.searchQuery)
	m.filteredColumns = nil

	for _, col := range m.mysqlColumns {
		if strings.Contains(strings.ToLower(col.Name), query) ||
			strings.Contains(strings.ToLower(col.DataType), query) {
			m.filteredColumns = append(m.filteredColumns, col)
		}
	}

	// Reset cursor if out of bounds
	if m.columnCursor >= len(m.filteredColumns) {
		m.columnCursor = 0
	}
}

// filterTables filters tables based on search query
func (m *Model) filterTables(tables []db.TableInfo) {
	if m.tableSearchQuery == "" {
		m.filteredTables = nil
		return
	}

	query := strings.ToLower(m.tableSearchQuery)
	m.filteredTables = nil

	for _, table := range tables {
		if strings.Contains(strings.ToLower(table.Name), query) {
			m.filteredTables = append(m.filteredTables, table)
		}
	}

	// Reset cursor if out of bounds
	if m.tableCursor >= len(m.filteredTables) {
		m.tableCursor = 0
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
