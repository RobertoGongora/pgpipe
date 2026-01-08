package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

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
