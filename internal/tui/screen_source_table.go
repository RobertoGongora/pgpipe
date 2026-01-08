package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// viewSourceTable renders the MySQL source table selection screen
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

// handleSourceTableKeys handles key presses on the source table screen
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
