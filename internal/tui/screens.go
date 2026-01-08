package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// ============================================================================
// Welcome Screen
// ============================================================================

func (m Model) viewWelcome() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("pgpipe"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render("MySQL to PostgreSQL Migration Tool"))
	sb.WriteString("\n\n")

	// Check what we have saved
	hasConfig := m.config.Migration.Source.Table != ""
	hasState := m.hasExistingState && m.state != nil

	if hasState {
		// Active migration found - show progress
		sb.WriteString(styles.Box.Render(fmt.Sprintf(
			"Existing migration found!\n\n"+
				"Source: %s\n"+
				"Progress: %s / %s (%.1f%%)\n"+
				"Last run: %s",
			m.state.Source.Table,
			styles.FormatNumber(m.state.Progress.ProcessedRows),
			styles.FormatNumber(m.state.Source.TotalRows),
			m.state.ProgressPercent(),
			m.state.LastRun.EndedAt.Format("2006-01-02 15:04"),
		)))
		sb.WriteString("\n\n")

		options := []string{"Resume migration", "Start new migration"}
		for i, opt := range options {
			if i == m.resumeChoice {
				sb.WriteString(styles.SelectedItem.Render("▸ " + opt))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + opt))
			}
			sb.WriteString("\n")
		}
	} else if hasConfig {
		// Saved config found but no active migration
		sb.WriteString(styles.Box.Render(fmt.Sprintf(
			"Saved configuration found!\n\n"+
				"Source: %s → %s\n"+
				"Columns: %d mapped\n"+
				"Batch size: %d",
			m.config.Migration.Source.Table,
			m.config.Migration.Target.Table,
			len(m.config.Migration.Mapping),
			m.config.Migration.Settings.BatchSize,
		)))
		sb.WriteString("\n\n")

		options := []string{"Use saved configuration", "Start fresh"}
		for i, opt := range options {
			if i == m.resumeChoice {
				sb.WriteString(styles.SelectedItem.Render("▸ " + opt))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + opt))
			}
			sb.WriteString("\n")
		}
	} else {
		// No config or state
		sb.WriteString("No existing configuration found.\n")
		sb.WriteString("Press Enter to start a new migration.\n")
	}

	sb.WriteString("\n")
	sb.WriteString(renderHelp(
		helpItem{Key: "↑/↓", Description: "Navigate"},
		helpItem{Key: "Enter", Description: "Select"},
		helpItem{Key: "q", Description: "Quit"},
	))

	return sb.String()
}

func (m Model) handleWelcomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	hasConfig := m.config.Migration.Source.Table != ""
	hasState := m.hasExistingState && m.state != nil

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.resumeChoice > 0 {
			m.resumeChoice--
		}
	case "down", "j":
		if (hasState || hasConfig) && m.resumeChoice < 1 {
			m.resumeChoice++
		}
	case "enter":
		if hasState && m.resumeChoice == 0 {
			// Resume existing migration - jump to settings
			// Config and state are already loaded in NewModel()
			m.screen = ScreenSettings
			return m, m.connectDatabases
		} else if hasConfig && m.resumeChoice == 0 {
			// Use saved config - config is already loaded in NewModel()
			// Jump to settings since everything is pre-configured
			m.screen = ScreenSettings
			return m, m.connectDatabases
		} else if (hasState || hasConfig) && m.resumeChoice == 1 {
			// Start fresh - delete state and config
			migration.DeleteState()
			m.state = nil
			m.hasExistingState = false

			// Reset config to defaults
			m.config = config.NewDefaultConfig()
			m.config.Save()

			// Reset all selections
			m.selectedColumns = make(map[string]bool)
			m.sourceTable = ""
			m.targetTable = ""
			m.columnMappings = nil
			m.batchSize = 5000
			m.batchLimit = 100

			m.screen = ScreenConnections
			return m, m.connectDatabases
		} else {
			// No existing state or config - start new
			m.screen = ScreenConnections
			return m, m.connectDatabases
		}
	}
	return m, nil
}

// ============================================================================
// Connections Screen
// ============================================================================

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

func (m Model) handleConnectionTest(msg ConnectionTestMsg) (Model, tea.Cmd) {
	if msg.mysqlErr != nil {
		m.mysqlError = msg.mysqlErr.Error()
		m.mysqlConnected = false
	} else {
		m.mysqlConnected = true
		m.mysqlError = ""
		// Store client
		client, _ := db.NewMySQLClient(&m.config.MySQL)
		m.mysqlClient = client
	}

	if msg.pgErr != nil {
		m.pgError = msg.pgErr.Error()
		m.pgConnected = false
	} else {
		m.pgConnected = true
		m.pgError = ""
		// Store client
		client, _ := db.NewPostgresClient(&m.config.PostgreSQL)
		m.pgClient = client
	}

	// Load tables if both connected
	if m.mysqlConnected && m.pgConnected {
		// Check if we're resuming with pre-configured tables
		if m.sourceTable != "" && m.targetTable != "" && len(m.columnMappings) > 0 {
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

func (m Model) loadMySQLTables() tea.Msg {
	if m.mysqlClient == nil {
		return MySQLTablesMsg{err: fmt.Errorf("not connected")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	tables, err := m.mysqlClient.GetTables(ctx)
	return MySQLTablesMsg{tables: tables, err: err}
}

func (m Model) loadPGTables() tea.Msg {
	if m.pgClient == nil {
		return PGTablesMsg{err: fmt.Errorf("not connected")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	tables, err := m.pgClient.GetTables(ctx)
	return PGTablesMsg{tables: tables, err: err}
}

func (m Model) viewConnections() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Database Connections"))
	sb.WriteString("\n\n")

	// MySQL status
	mysqlStatus := "⏳ Connecting..."
	mysqlStyle := styles.StatusMuted
	if m.mysqlConnected {
		mysqlStatus = "✓ Connected"
		mysqlStyle = styles.StatusSuccess
	} else if m.mysqlError != "" {
		mysqlStatus = "✗ " + truncate(m.mysqlError, 50)
		mysqlStyle = styles.StatusError
	}

	sb.WriteString(styles.Box.Render(fmt.Sprintf(
		"MySQL\n"+
			"├─ Host: %s:%d\n"+
			"├─ Database: %s\n"+
			"└─ Status: %s",
		m.config.MySQL.Host,
		m.config.MySQL.Port,
		m.config.MySQL.Database,
		mysqlStyle.Render(mysqlStatus),
	)))
	sb.WriteString("\n\n")

	// PostgreSQL status
	pgStatus := "⏳ Connecting..."
	pgStyle := styles.StatusMuted
	if m.pgConnected {
		pgStatus = "✓ Connected"
		pgStyle = styles.StatusSuccess
	} else if m.pgError != "" {
		pgStatus = "✗ " + truncate(m.pgError, 50)
		pgStyle = styles.StatusError
	}

	sb.WriteString(styles.Box.Render(fmt.Sprintf(
		"PostgreSQL\n"+
			"├─ Host: %s:%d\n"+
			"├─ Database: %s\n"+
			"└─ Status: %s",
		m.config.PostgreSQL.Host,
		m.config.PostgreSQL.Port,
		m.config.PostgreSQL.Database,
		pgStyle.Render(pgStatus),
	)))
	sb.WriteString("\n\n")

	if m.mysqlConnected && m.pgConnected {
		sb.WriteString(renderHelp(
			helpItem{Key: "Enter", Description: "Continue"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else if m.mysqlError != "" || m.pgError != "" {
		sb.WriteString(renderHelp(
			helpItem{Key: "r", Description: "Retry"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(styles.Help.Render("Connecting..."))
	}

	return sb.String()
}

func (m Model) handleConnectionsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "r":
		return m, m.connectDatabases
	case "enter":
		if m.mysqlConnected && m.pgConnected {
			m.screen = ScreenSourceTable
		}
	}
	return m, nil
}

// ============================================================================
// Source Table Screen
// ============================================================================

func (m Model) viewSourceTable() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Source Table (MySQL)"))
	sb.WriteString("\n\n")

	if len(m.mysqlTables) == 0 {
		sb.WriteString("Loading tables...\n")
		return sb.String()
	}

	// Show search box if in search mode
	if m.tableSearchMode {
		searchBox := fmt.Sprintf("Search: %s_", m.tableSearchQuery)
		sb.WriteString(styles.InputFocused.Render(searchBox))
		sb.WriteString("\n\n")
	}

	// Use filtered tables if searching, otherwise use all tables
	tables := m.mysqlTables
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 {
		tables = m.filteredTables
	}

	// Show "no results" message if search returned nothing
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 && len(tables) == 0 {
		sb.WriteString(styles.StatusWarning.Render("No tables match your search\n"))
		sb.WriteString("\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Backspace", Description: "Clear search"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "q", Description: "Quit"},
		))
		return sb.String()
	}

	// Calculate visible range
	visibleCount := MaxVisibleTables
	startIdx := 0
	if m.tableCursor >= visibleCount {
		startIdx = m.tableCursor - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(tables) {
		endIdx = len(tables)
	}

	for i := startIdx; i < endIdx; i++ {
		t := tables[i]
		line := fmt.Sprintf("%-30s %s rows", t.Name, styles.FormatNumber(t.RowCount))
		if i == m.tableCursor {
			sb.WriteString(styles.SelectedItem.Render("▸ " + line))
		} else {
			sb.WriteString(styles.ListItem.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	if len(tables) > visibleCount {
		sb.WriteString(fmt.Sprintf("\n(%d/%d tables)", m.tableCursor+1, len(tables)))
	}
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 {
		sb.WriteString(fmt.Sprintf(" (%d matches)", len(tables)))
	}

	sb.WriteString("\n")

	// Different help text depending on mode
	if m.tableSearchMode {
		sb.WriteString(renderHelp(
			helpItem{Key: "Type", Description: "Search"},
			helpItem{Key: "Backspace", Description: "Delete"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(renderHelp(
			helpItem{Key: "/", Description: "Search"},
			helpItem{Key: "↑/↓", Description: "Navigate"},
			helpItem{Key: "Enter", Description: "Select"},
			helpItem{Key: "q", Description: "Quit"},
		))
	}

	return sb.String()
}

func (m Model) handleSourceTableKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode
	if m.tableSearchMode {
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Exit search mode
			m.tableSearchMode = false
			m.tableSearchQuery = ""
			m.filteredTables = nil
			m.tableCursor = 0
		case "backspace":
			if len(m.tableSearchQuery) > 0 {
				m.tableSearchQuery = m.tableSearchQuery[:len(m.tableSearchQuery)-1]
				m.filterTables(m.mysqlTables)
			}
		case "up", "k":
			if m.tableCursor > 0 {
				m.tableCursor--
			}
		case "down", "j":
			tables := m.filteredTables
			if len(tables) > 0 && m.tableCursor < len(tables)-1 {
				m.tableCursor++
			}
		case "enter":
			// Select from filtered results
			if len(m.filteredTables) > 0 && m.tableCursor < len(m.filteredTables) {
				m.sourceTable = m.filteredTables[m.tableCursor].Name
				// Find original index in mysqlTables
				for i, t := range m.mysqlTables {
					if t.Name == m.sourceTable {
						m.sourceTableIdx = i
						break
					}
				}

				// Save source table selection
				m.config.Migration.Source.Table = m.sourceTable
				m.config.Save()

				m.screen = ScreenSourceColumns
				// Exit search mode
				m.tableSearchMode = false
				m.tableSearchQuery = ""
				m.filteredTables = nil
				return m, m.loadMySQLColumns
			}
		default:
			// Add character to search query
			if len(msg.String()) == 1 && msg.String() != "/" {
				m.tableSearchQuery += msg.String()
				m.filterTables(m.mysqlTables)
			}
		}
		return m, nil
	}

	// Normal mode handling
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "/":
		// Enter search mode
		m.tableSearchMode = true
		m.tableSearchQuery = ""
		m.filteredTables = nil
		m.tableCursor = 0
	case "up", "k":
		if m.tableCursor > 0 {
			m.tableCursor--
		}
	case "down", "j":
		if m.tableCursor < len(m.mysqlTables)-1 {
			m.tableCursor++
		}
	case "enter":
		if len(m.mysqlTables) > 0 {
			m.sourceTable = m.mysqlTables[m.tableCursor].Name
			m.sourceTableIdx = m.tableCursor

			// Save source table selection
			m.config.Migration.Source.Table = m.sourceTable
			m.config.Save()

			m.screen = ScreenSourceColumns
			return m, m.loadMySQLColumns
		}
	}
	return m, nil
}

func (m Model) loadMySQLColumns() tea.Msg {
	if m.mysqlClient == nil || m.sourceTable == "" {
		return MySQLColumnsMsg{err: fmt.Errorf("not ready")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	columns, err := m.mysqlClient.GetColumns(ctx, m.sourceTable)
	return MySQLColumnsMsg{columns: columns, err: err}
}

// ============================================================================
// Source Columns Screen
// ============================================================================

func (m Model) viewSourceColumns() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Columns to Migrate"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Table: %s", m.sourceTable)))
	sb.WriteString("\n\n")

	if len(m.mysqlColumns) == 0 {
		sb.WriteString("Loading columns...\n")
		return sb.String()
	}

	// Show search box if in search mode
	if m.searchMode {
		searchBox := fmt.Sprintf("Search: %s_", m.searchQuery)
		sb.WriteString(styles.InputFocused.Render(searchBox))
		sb.WriteString("\n\n")
	}

	// Find primary key
	var pkName string
	for _, col := range m.mysqlColumns {
		if col.IsPrimaryKey {
			pkName = col.Name
			break
		}
	}
	sb.WriteString(styles.Label.Render(fmt.Sprintf("Primary Key: %s (auto-included)\n\n", pkName)))

	// Use filtered columns if searching, otherwise use all columns
	columns := m.mysqlColumns
	if m.searchMode && len(m.searchQuery) > 0 {
		columns = m.filteredColumns
	}

	// Show "no results" message if search returned nothing
	if m.searchMode && len(m.searchQuery) > 0 && len(columns) == 0 {
		sb.WriteString(styles.StatusWarning.Render("No columns match your search\n"))
		sb.WriteString("\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Backspace", Description: "Clear search"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "q", Description: "Quit"},
		))
		return sb.String()
	}

	// Implement scrolling viewport
	visibleCount := MaxVisibleColumns

	// Calculate visible range centered on cursor
	startIdx := m.columnCursor - visibleCount/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(columns) {
		endIdx = len(columns)
		startIdx = endIdx - visibleCount
		if startIdx < 0 {
			startIdx = 0
		}
	}

	// Render visible columns only
	for i := startIdx; i < endIdx; i++ {
		col := columns[i]
		if col.IsPrimaryKey {
			continue // Skip PK in selection
		}

		checkbox := "[ ]"
		if m.selectedColumns[col.Name] {
			checkbox = styles.CheckboxChecked.Render("[✓]")
		}

		line := fmt.Sprintf("%s %-25s %s", checkbox, col.Name, col.DataType)
		if i == m.columnCursor {
			sb.WriteString(styles.SelectedItem.Render("▸ " + line))
		} else {
			sb.WriteString(styles.ListItem.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	// Show position indicator if list is longer than viewport
	if len(columns) > visibleCount {
		sb.WriteString(fmt.Sprintf("\nShowing %d-%d of %d columns\n",
			startIdx+1, endIdx, len(columns)))
	}

	// Count selected
	selected := 0
	for _, v := range m.selectedColumns {
		if v {
			selected++
		}
	}

	sb.WriteString(fmt.Sprintf("\n%d columns selected", selected))
	if m.searchMode && len(m.searchQuery) > 0 {
		sb.WriteString(fmt.Sprintf(" (%d matches)\n", len(columns)))
	} else {
		sb.WriteString("\n")
	}

	// Different help text depending on mode
	if m.searchMode {
		sb.WriteString(renderHelp(
			helpItem{Key: "Type", Description: "Search"},
			helpItem{Key: "Backspace", Description: "Delete"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "Space", Description: "Toggle"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(renderHelp(
			helpItem{Key: "/", Description: "Search"},
			helpItem{Key: "↑/↓", Description: "Navigate"},
			helpItem{Key: "Space", Description: "Toggle"},
			helpItem{Key: "Enter", Description: "Continue"},
			helpItem{Key: "a", Description: "All"},
			helpItem{Key: "n", Description: "None"},
			helpItem{Key: "q", Description: "Quit"},
		))
	}

	return sb.String()
}

func (m Model) handleSourceColumnsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode
	if m.searchMode {
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Exit search mode
			m.searchMode = false
			m.searchQuery = ""
			m.filteredColumns = nil
			m.columnCursor = 0
		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.filterColumns()
			}
		case "up", "k":
			// Navigate in filtered results
			if m.columnCursor > 0 {
				m.columnCursor--
			}
		case "down", "j":
			// Navigate in filtered results
			columns := m.filteredColumns
			if len(columns) > 0 && m.columnCursor < len(columns)-1 {
				m.columnCursor++
			}
		case " ":
			// Toggle selection in filtered results
			if len(m.filteredColumns) > 0 && m.columnCursor < len(m.filteredColumns) {
				col := m.filteredColumns[m.columnCursor]
				if !col.IsPrimaryKey {
					m.selectedColumns[col.Name] = !m.selectedColumns[col.Name]
				}
			}
		default:
			// Add character to search query (only single chars)
			if len(msg.String()) == 1 && msg.String() != "/" {
				m.searchQuery += msg.String()
				m.filterColumns()
			}
		}
		return m, nil
	}

	// Normal mode handling
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "/":
		// Enter search mode
		m.searchMode = true
		m.searchQuery = ""
		m.filteredColumns = nil
		m.columnCursor = 0
	case "up", "k":
		if m.columnCursor > 0 {
			m.columnCursor--
		}
	case "down", "j":
		if m.columnCursor < len(m.mysqlColumns)-1 {
			m.columnCursor++
		}
	case " ":
		if len(m.mysqlColumns) > 0 {
			col := m.mysqlColumns[m.columnCursor]
			if !col.IsPrimaryKey {
				m.selectedColumns[col.Name] = !m.selectedColumns[col.Name]
			}
		}
	case "a":
		for _, col := range m.mysqlColumns {
			if !col.IsPrimaryKey {
				m.selectedColumns[col.Name] = true
			}
		}
	case "n":
		m.selectedColumns = make(map[string]bool)
	case "enter":
		// Check at least one column selected
		hasSelection := false
		for _, v := range m.selectedColumns {
			if v {
				hasSelection = true
				break
			}
		}
		if hasSelection {
			// Build selected columns list
			var selectedCols []string
			for _, col := range m.mysqlColumns {
				if m.selectedColumns[col.Name] && !col.IsPrimaryKey {
					selectedCols = append(selectedCols, col.Name)
				}
			}

			// Get PK
			var pkCol string
			for _, col := range m.mysqlColumns {
				if col.IsPrimaryKey {
					pkCol = col.Name
					break
				}
			}

			// Save column selection
			m.config.Migration.Source.Columns = selectedCols
			m.config.Migration.Source.PrimaryKey = pkCol
			m.config.Save()

			m.tableCursor = 0 // Reset for target table selection
			m.screen = ScreenTargetTable
		}
	}
	return m, nil
}

// ============================================================================
// Target Table Screen
// ============================================================================

func (m Model) viewTargetTable() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Target Table (PostgreSQL)"))
	sb.WriteString("\n\n")

	if len(m.pgTables) == 0 {
		sb.WriteString("Loading tables...\n")
		return sb.String()
	}

	// Show search box if in search mode
	if m.tableSearchMode {
		searchBox := fmt.Sprintf("Search: %s_", m.tableSearchQuery)
		sb.WriteString(styles.InputFocused.Render(searchBox))
		sb.WriteString("\n\n")
	}

	// Use filtered tables if searching, otherwise use all tables
	tables := m.pgTables
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 {
		tables = m.filteredTables
	}

	// Show "no results" message if search returned nothing
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 && len(tables) == 0 {
		sb.WriteString(styles.StatusWarning.Render("No tables match your search\n"))
		sb.WriteString("\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Backspace", Description: "Clear search"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "q", Description: "Quit"},
		))
		return sb.String()
	}

	visibleCount := MaxVisibleTables
	startIdx := 0
	if m.tableCursor >= visibleCount {
		startIdx = m.tableCursor - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(tables) {
		endIdx = len(tables)
	}

	for i := startIdx; i < endIdx; i++ {
		t := tables[i]
		line := fmt.Sprintf("%-40s %s rows", t.Name, styles.FormatNumber(t.RowCount))
		if i == m.tableCursor {
			sb.WriteString(styles.SelectedItem.Render("▸ " + line))
		} else {
			sb.WriteString(styles.ListItem.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	if len(tables) > visibleCount {
		sb.WriteString(fmt.Sprintf("\n(%d/%d tables)", m.tableCursor+1, len(tables)))
	}
	if m.tableSearchMode && len(m.tableSearchQuery) > 0 {
		sb.WriteString(fmt.Sprintf(" (%d matches)", len(tables)))
	}

	sb.WriteString("\n")

	// Different help text depending on mode
	if m.tableSearchMode {
		sb.WriteString(renderHelp(
			helpItem{Key: "Type", Description: "Search"},
			helpItem{Key: "Backspace", Description: "Delete"},
			helpItem{Key: "Esc", Description: "Exit search"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(renderHelp(
			helpItem{Key: "/", Description: "Search"},
			helpItem{Key: "↑/↓", Description: "Navigate"},
			helpItem{Key: "Enter", Description: "Select"},
			helpItem{Key: "Esc", Description: "Back"},
			helpItem{Key: "q", Description: "Quit"},
		))
	}

	return sb.String()
}

func (m Model) handleTargetTableKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode
	if m.tableSearchMode {
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Exit search mode if in search, otherwise go back to previous screen
			if m.tableSearchQuery != "" {
				m.tableSearchMode = false
				m.tableSearchQuery = ""
				m.filteredTables = nil
				m.tableCursor = 0
			} else {
				m.tableSearchMode = false
				m.screen = ScreenSourceColumns
			}
		case "backspace":
			if len(m.tableSearchQuery) > 0 {
				m.tableSearchQuery = m.tableSearchQuery[:len(m.tableSearchQuery)-1]
				m.filterTables(m.pgTables)
			}
		case "up", "k":
			if m.tableCursor > 0 {
				m.tableCursor--
			}
		case "down", "j":
			tables := m.filteredTables
			if len(tables) > 0 && m.tableCursor < len(tables)-1 {
				m.tableCursor++
			}
		case "enter":
			// Select from filtered results
			if len(m.filteredTables) > 0 && m.tableCursor < len(m.filteredTables) {
				m.targetTable = m.filteredTables[m.tableCursor].Name
				// Find original index in pgTables
				for i, t := range m.pgTables {
					if t.Name == m.targetTable {
						m.targetTableIdx = i
						break
					}
				}

				// Save target table selection
				m.config.Migration.Target.Table = m.targetTable
				m.config.Save()

				m.screen = ScreenMapping
				// Exit search mode
				m.tableSearchMode = false
				m.tableSearchQuery = ""
				m.filteredTables = nil
				return m, m.loadPGColumns
			}
		default:
			// Add character to search query
			if len(msg.String()) == 1 && msg.String() != "/" {
				m.tableSearchQuery += msg.String()
				m.filterTables(m.pgTables)
			}
		}
		return m, nil
	}

	// Normal mode handling
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = ScreenSourceColumns
	case "/":
		// Enter search mode
		m.tableSearchMode = true
		m.tableSearchQuery = ""
		m.filteredTables = nil
		m.tableCursor = 0
	case "up", "k":
		if m.tableCursor > 0 {
			m.tableCursor--
		}
	case "down", "j":
		if m.tableCursor < len(m.pgTables)-1 {
			m.tableCursor++
		}
	case "enter":
		if len(m.pgTables) > 0 {
			m.targetTable = m.pgTables[m.tableCursor].Name
			m.targetTableIdx = m.tableCursor

			// Save target table selection
			m.config.Migration.Target.Table = m.targetTable
			m.config.Save()

			m.screen = ScreenMapping
			return m, m.loadPGColumns
		}
	}
	return m, nil
}

func (m Model) loadPGColumns() tea.Msg {
	if m.pgClient == nil || m.targetTable == "" {
		return PGColumnsMsg{err: fmt.Errorf("not ready")}
	}
	ctx, cancel := context.WithTimeout(context.Background(), QueryTimeout)
	defer cancel()
	columns, err := m.pgClient.GetColumns(ctx, m.targetTable)
	return PGColumnsMsg{columns: columns, err: err}
}

// ============================================================================
// Column Mapping Screen
// ============================================================================

func (m Model) viewMapping() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Column Mapping"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("%s → %s", m.sourceTable, m.targetTable)))
	sb.WriteString("\n\n")

	if len(m.columnMappings) == 0 {
		sb.WriteString("Generating mappings...\n")
	} else {
		// Header
		header := fmt.Sprintf("%-25s %-25s %-15s", "Source", "Target", "Transform")
		sb.WriteString(styles.TableHeader.Render(header))
		sb.WriteString("\n")

		for i, mapping := range m.columnMappings {
			target := mapping.Target
			if target == "" {
				target = "(skip)"
			}
			transform := mapping.Transform
			if transform == "" {
				transform = "-"
			}

			line := fmt.Sprintf("%-25s %-25s %-15s", mapping.Source, target, transform)
			if i == m.mappingCursor {
				sb.WriteString(styles.SelectedItem.Render("▸ " + line))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	// Show warnings
	var warnings []string
	for _, mapping := range m.columnMappings {
		if mapping.Transform == "text_to_jsonb" {
			warnings = append(warnings, fmt.Sprintf("⚠ %s: TEXT → JSONB (invalid JSON will be skipped)", mapping.Source))
		}
	}
	if len(warnings) > 0 {
		sb.WriteString("\n")
		sb.WriteString(styles.Box.Render(strings.Join(warnings, "\n")))
	}

	sb.WriteString("\n")
	sb.WriteString(renderHelp(
		helpItem{Key: "↑/↓", Description: "Navigate"},
		helpItem{Key: "Enter", Description: "Edit"},
		helpItem{Key: "c", Description: "Continue"},
		helpItem{Key: "Esc", Description: "Back"},
		helpItem{Key: "q", Description: "Quit"},
	))

	return sb.String()
}

func (m Model) handleMappingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = ScreenTargetTable
	case "up", "k":
		if m.mappingCursor > 0 {
			m.mappingCursor--
		}
	case "down", "j":
		if m.mappingCursor < len(m.columnMappings)-1 {
			m.mappingCursor++
		}
	case "enter":
		// Enter edit mode for the selected mapping
		m.editingMapping = true
		m.editTargetCursor = 0
		// Set available targets from pgColumns
		m.availableTargets = m.pgColumns

		// Pre-select current target if one is set
		if m.mappingCursor < len(m.columnMappings) {
			currentTarget := m.columnMappings[m.mappingCursor].Target
			if currentTarget != "" {
				// Find the index of the current target
				for i, col := range m.availableTargets {
					if col.Name == currentTarget {
						m.editTargetCursor = i + 1 // +1 because 0 is skip option
						break
					}
				}
			}
		}
	case "c":
		// Save mappings before continuing
		m.config.Migration.Mapping = m.columnMappings
		m.config.Save()

		// Continue to settings
		m.screen = ScreenSettings
	}
	return m, nil
}

// ============================================================================
// Mapping Editor (Modal)
// ============================================================================

func (m Model) viewMappingEditor() string {
	var sb strings.Builder

	if m.mappingCursor >= len(m.columnMappings) {
		return "Invalid mapping selection"
	}

	currentMapping := m.columnMappings[m.mappingCursor]

	sb.WriteString(styles.Title.Render("Edit Column Mapping"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Source: %s (%s)",
		currentMapping.Source,
		m.getSourceColumnType(currentMapping.Source))))
	sb.WriteString("\n\n")

	sb.WriteString("Select target column:\n\n")

	// Calculate scrolling viewport for target columns
	// +1 for skip option at index 0
	totalOptions := len(m.availableTargets) + 1
	visibleCount := MaxVisibleTargets

	startIdx := m.editTargetCursor - visibleCount/2
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + visibleCount
	if endIdx > totalOptions {
		endIdx = totalOptions
		startIdx = endIdx - visibleCount
		if startIdx < 0 {
			startIdx = 0
		}
	}

	// Render visible options
	for i := startIdx; i < endIdx; i++ {
		var line string
		if i == 0 {
			// Skip option
			line = "(skip) - Don't migrate this column"
		} else {
			// Target column option
			col := m.availableTargets[i-1]
			line = fmt.Sprintf("%-30s %s", col.Name, col.DataType)

			// Show if this column is already mapped from another source
			mappedFrom := m.getSourceMappedTo(col.Name)
			if mappedFrom != "" && mappedFrom != currentMapping.Source {
				line += fmt.Sprintf(" (mapped from: %s)", mappedFrom)
			}
		}

		if i == m.editTargetCursor {
			sb.WriteString(styles.SelectedItem.Render("▸ " + line))
		} else {
			sb.WriteString(styles.ListItem.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	if totalOptions > visibleCount {
		sb.WriteString(fmt.Sprintf("\nShowing %d-%d of %d options\n",
			startIdx+1, endIdx, totalOptions))
	}

	sb.WriteString("\n")
	sb.WriteString(renderHelp(
		helpItem{Key: "↑/↓", Description: "Navigate"},
		helpItem{Key: "Enter", Description: "Select"},
		helpItem{Key: "Esc", Description: "Cancel"},
		helpItem{Key: "q", Description: "Quit"},
	))

	return sb.String()
}

func (m Model) handleMappingEditorKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxCursor := len(m.availableTargets) // +1 for skip at 0, but 0-indexed so it works out

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		// Cancel editing
		m.editingMapping = false
		m.editTargetCursor = 0
	case "up", "k":
		if m.editTargetCursor > 0 {
			m.editTargetCursor--
		}
	case "down", "j":
		if m.editTargetCursor < maxCursor {
			m.editTargetCursor++
		}
	case "enter":
		// Apply the selection
		if m.editTargetCursor == 0 {
			// Skip option selected
			m.columnMappings[m.mappingCursor].Target = ""
			m.columnMappings[m.mappingCursor].Transform = ""
		} else {
			// Target column selected
			selectedCol := m.availableTargets[m.editTargetCursor-1]
			m.columnMappings[m.mappingCursor].Target = selectedCol.Name

			// Auto-detect transform if TEXT->JSONB
			sourceCol := m.getSourceColumn(m.columnMappings[m.mappingCursor].Source)
			if sourceCol != nil && isTextType(sourceCol.DataType) && isJSONType(selectedCol.DataType) {
				m.columnMappings[m.mappingCursor].Transform = "text_to_jsonb"
			} else {
				m.columnMappings[m.mappingCursor].Transform = ""
			}
		}

		// Save updated mappings
		m.config.Migration.Mapping = m.columnMappings
		m.config.Save()

		// Exit edit mode
		m.editingMapping = false
		m.editTargetCursor = 0
	}
	return m, nil
}

// Helper methods for mapping editor

func (m Model) getSourceColumnType(colName string) string {
	for _, col := range m.mysqlColumns {
		if col.Name == colName {
			return col.DataType
		}
	}
	return "unknown"
}

func (m Model) getSourceColumn(colName string) *db.ColumnInfo {
	for _, col := range m.mysqlColumns {
		if col.Name == colName {
			return &col
		}
	}
	return nil
}

func (m Model) getSourceMappedTo(targetColName string) string {
	for _, mapping := range m.columnMappings {
		if mapping.Target == targetColName {
			return mapping.Source
		}
	}
	return ""
}

// ============================================================================
// Settings Screen
// ============================================================================

func (m Model) viewSettings() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration Settings"))
	sb.WriteString("\n\n")

	// Batch size
	var batchLine string
	if m.editingBatchSize {
		batchLine = fmt.Sprintf("Batch Size: [%s_]", m.inputBuffer)
	} else {
		batchLine = fmt.Sprintf("Batch Size: %d", m.batchSize)
	}
	if m.settingsCursor == 0 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + batchLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + batchLine))
	}
	sb.WriteString("\n\n")

	// Run mode
	sb.WriteString("Run Mode:\n")

	continuousLine := "○ Continuous - Run until complete"
	if m.runMode == migration.RunModeContinuous {
		continuousLine = "● Continuous - Run until complete"
	}
	if m.settingsCursor == 1 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + continuousLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + continuousLine))
	}
	sb.WriteString("\n")

	var batchesLine string
	if m.editingBatchLimit {
		batchesLine = fmt.Sprintf("● Run [%s_] batches then stop", m.inputBuffer)
	} else {
		batchesLine = fmt.Sprintf("○ Run %d batches then stop", m.batchLimit)
	}
	if m.runMode == migration.RunModeBatches {
		if !m.editingBatchLimit {
			batchesLine = fmt.Sprintf("● Run %d batches then stop", m.batchLimit)
		}
	}
	if m.settingsCursor == 2 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + batchesLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + batchesLine))
	}
	sb.WriteString("\n\n")

	// Summary
	if len(m.mysqlTables) > 0 {
		totalRows := m.mysqlTables[m.sourceTableIdx].RowCount
		estimatedBatches := (totalRows + int64(m.batchSize) - 1) / int64(m.batchSize)

		var willProcess string
		if m.runMode == migration.RunModeContinuous {
			willProcess = fmt.Sprintf("%s rows", styles.FormatNumber(totalRows))
		} else {
			rowsThisRun := int64(m.batchLimit) * int64(m.batchSize)
			if rowsThisRun > totalRows {
				rowsThisRun = totalRows
			}
			willProcess = fmt.Sprintf("~%s rows", styles.FormatNumber(rowsThisRun))
		}

		sb.WriteString(styles.Box.Render(fmt.Sprintf(
			"Summary\n"+
				"Total rows: %s\n"+
				"Estimated batches: %d\n"+
				"This run will process: %s",
			styles.FormatNumber(totalRows),
			estimatedBatches,
			willProcess,
		)))
	}

	sb.WriteString("\n\n")

	// Add "Start Migration" button
	startLine := fmt.Sprintf("▶ Start Migration (press %s)", "s")
	if m.settingsCursor == 3 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + startLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + startLine))
	}

	sb.WriteString("\n\n")

	// Different help text depending on mode
	if m.editingBatchSize || m.editingBatchLimit {
		sb.WriteString(renderHelp(
			helpItem{Key: "Type", Description: "Number"},
			helpItem{Key: "Backspace", Description: "Delete"},
			helpItem{Key: "Enter", Description: "Confirm"},
			helpItem{Key: "Esc", Description: "Cancel"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(renderHelp(
			helpItem{Key: "↑/↓", Description: "Navigate"},
			helpItem{Key: "←/→", Description: "Adjust"},
			helpItem{Key: "Enter", Description: "Edit"},
			helpItem{Key: "Space", Description: "Toggle"},
			helpItem{Key: "s", Description: "Start migration"},
			helpItem{Key: "Esc", Description: "Back"},
			helpItem{Key: "q", Description: "Quit"},
		))
	}

	return sb.String()
}

func (m Model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle batch size editing mode
	if m.editingBatchSize {
		switch msg.String() {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.inputBuffer += msg.String()
		case "backspace":
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
		case "enter":
			// Apply value with validation
			if val, err := strconv.Atoi(m.inputBuffer); err == nil {
				if val < MinBatchSize {
					val = MinBatchSize
				}
				if val > MaxBatchSize {
					val = MaxBatchSize
				}
				m.batchSize = val
			}
			m.editingBatchSize = false
			m.inputBuffer = ""
		case "esc":
			// Cancel editing
			m.editingBatchSize = false
			m.inputBuffer = ""
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Handle batch limit editing mode
	if m.editingBatchLimit {
		switch msg.String() {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.inputBuffer += msg.String()
		case "backspace":
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
		case "enter":
			// Apply value with validation
			if val, err := strconv.Atoi(m.inputBuffer); err == nil {
				if val < MinBatchLimit {
					val = MinBatchLimit
				}
				if val > MaxBatchLimit {
					val = MaxBatchLimit
				}
				m.batchLimit = val
			}
			m.editingBatchLimit = false
			m.inputBuffer = ""
		case "esc":
			// Cancel editing
			m.editingBatchLimit = false
			m.inputBuffer = ""
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Normal mode
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		m.screen = ScreenMapping
	case "up", "k":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "j":
		if m.settingsCursor < 3 { // Now 3 because we added "Start Migration"
			m.settingsCursor++
		}
	case "left", "h":
		switch m.settingsCursor {
		case 0: // Batch size
			if m.batchSize > 100 {
				m.batchSize -= 100
			} else if m.batchSize > 1 {
				m.batchSize = 1 // Jump to minimum
			}
		case 2: // Batch limit
			if m.batchLimit > 10 {
				m.batchLimit -= 10
			} else if m.batchLimit > 1 {
				m.batchLimit = 1 // Jump to minimum
			}
		}
	case "right", "l":
		switch m.settingsCursor {
		case 0: // Batch size
			if m.batchSize < 50000 {
				m.batchSize += 100
			}
		case 2: // Batch limit
			if m.batchLimit < 10000 {
				m.batchLimit += 10
			}
		}
	case "enter":
		if m.settingsCursor == 0 {
			// Edit batch size
			m.editingBatchSize = true
			m.inputBuffer = fmt.Sprintf("%d", m.batchSize)
		} else if m.settingsCursor == 1 {
			// Select continuous mode
			m.runMode = migration.RunModeContinuous
		} else if m.settingsCursor == 2 {
			// Edit batch limit or select batch mode
			m.runMode = migration.RunModeBatches
			m.editingBatchLimit = true
			m.inputBuffer = fmt.Sprintf("%d", m.batchLimit)
		}
	case "s":
		if !m.editingBatchSize && !m.editingBatchLimit {
			// Save final settings before starting
			m.config.Migration.Settings.BatchSize = m.batchSize
			m.config.Save()

			// Start migration
			m.screen = ScreenRunning
			return m, m.startMigration()
		}
	case " ":
		// Toggle run mode with space
		if m.settingsCursor == 1 {
			m.runMode = migration.RunModeContinuous
		} else if m.settingsCursor == 2 {
			m.runMode = migration.RunModeBatches
		}
	}
	return m, nil
}

// ============================================================================
// Running Screen
// ============================================================================

func (m Model) startMigration() tea.Cmd {
	// Capture all values we need from the model
	// This is necessary because the command runs asynchronously
	mysqlClient := m.mysqlClient
	pgClient := m.pgClient
	sourceTable := m.sourceTable
	targetTable := m.targetTable
	columnMappings := m.columnMappings
	batchSize := m.batchSize
	runMode := m.runMode
	batchLimit := m.batchLimit
	mysqlColumns := m.mysqlColumns
	existingState := m.state
	hasExistingState := m.hasExistingState

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

func (m Model) viewRunning() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration in Progress"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("%s → %s", m.sourceTable, m.targetTable)))
	sb.WriteString("\n\n")

	// Show initialization status if migrator not ready yet
	if m.migrator == nil {
		sb.WriteString(styles.Box.Render("Initializing Migration"))
		sb.WriteString("\n\n")

		if m.err != nil {
			// Show progress message (we reuse err field for this)
			sb.WriteString(m.err.Error())
			sb.WriteString("\n")
		} else {
			sb.WriteString("Preparing migration...\n")
			sb.WriteString("• Counting rows in source table\n")
			sb.WriteString("• Calculating ID range\n")
			sb.WriteString("• Initializing migrator\n")
		}

		sb.WriteString("\n")
		sb.WriteString(styles.Help.Render("Please wait..."))
		return sb.String()
	}

	if m.state == nil {
		sb.WriteString("Loading state...\n")
		return sb.String()
	}

	// Progress bar
	percent := m.state.ProgressPercent()
	sb.WriteString(progressBar(percent, 40))
	sb.WriteString("\n\n")

	// Stats
	stats := fmt.Sprintf(
		"Batches:     %d / %d\n"+
			"Rows:        %s / %s\n"+
			"Imported:    %s\n"+
			"Skipped:     %s\n"+
			"Cursor:      id > %d",
		m.state.Batches.Completed,
		m.state.EstimatedBatchesRemaining()+m.state.Batches.Completed,
		styles.FormatNumber(m.state.Progress.ProcessedRows),
		styles.FormatNumber(m.state.Source.TotalRows),
		styles.FormatNumber(m.state.Progress.ImportedRows),
		styles.FormatNumber(m.state.Progress.SkippedRows),
		m.state.Progress.LastCursor,
	)
	sb.WriteString(styles.Box.Render(stats))
	sb.WriteString("\n\n")

	// Check if done
	if m.state.IsComplete() {
		sb.WriteString(styles.StatusSuccess.Render("✓ Migration complete!"))
		sb.WriteString("\n\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Enter", Description: "View Summary"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(renderHelp(
			helpItem{Key: "q", Description: "Quit (progress saved)"},
		))
	}

	return sb.String()
}

func (m Model) handleRunningKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		if m.migrator != nil {
			m.migrator.Stop()
		}
		m.screen = ScreenSummary
	case "enter":
		if m.state != nil && m.state.IsComplete() {
			m.screen = ScreenSummary
		}
	}
	return m, nil
}

// ============================================================================
// Summary Screen
// ============================================================================

func (m Model) viewSummary() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration Summary"))
	sb.WriteString("\n\n")

	if m.state == nil {
		sb.WriteString("No migration data available.\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Enter", Description: "Exit"},
		))
		return sb.String()
	}

	// Status
	if m.state.IsComplete() {
		sb.WriteString(styles.StatusSuccess.Render("✓ Migration Complete!"))
	} else {
		sb.WriteString(styles.StatusWarning.Render("◐ Migration Paused"))
	}
	sb.WriteString("\n\n")

	// This run stats
	thisRun := fmt.Sprintf(
		"This Run\n"+
			"Duration:    %.1f seconds\n"+
			"Batches:     %d\n"+
			"Rows:        %s",
		m.state.LastRun.DurationSeconds,
		m.state.LastRun.BatchesCompleted,
		styles.FormatNumber(m.state.LastRun.RowsThisRun),
	)
	sb.WriteString(styles.Box.Render(thisRun))
	sb.WriteString("\n\n")

	// Overall progress
	overall := fmt.Sprintf(
		"Overall Progress\n"+
			"Progress:    %s / %s (%.1f%%)\n"+
			"Imported:    %s\n"+
			"Skipped:     %s\n"+
			"Remaining:   %s rows",
		styles.FormatNumber(m.state.Progress.ProcessedRows),
		styles.FormatNumber(m.state.Source.TotalRows),
		m.state.ProgressPercent(),
		styles.FormatNumber(m.state.Progress.ImportedRows),
		styles.FormatNumber(m.state.Progress.SkippedRows),
		styles.FormatNumber(m.state.RemainingRows()),
	)
	sb.WriteString(styles.Box.Render(overall))
	sb.WriteString("\n\n")

	// Error log
	if m.state.Progress.SkippedRows > 0 {
		errorInfo := fmt.Sprintf(
			"Errors\n"+
				"%d rows skipped\n"+
				"Log: %s",
			m.state.Progress.SkippedRows,
			m.state.Session.ErrorLog,
		)
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Warning).Render(styles.Box.Render(errorInfo)))
		sb.WriteString("\n\n")
	}

	if !m.state.IsComplete() {
		sb.WriteString("Run `pgpipe` again to continue migration.\n\n")
	}

	sb.WriteString(renderHelp(
		helpItem{Key: "Enter", Description: "Exit"},
	))

	return sb.String()
}

func (m Model) handleSummaryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "enter":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}
