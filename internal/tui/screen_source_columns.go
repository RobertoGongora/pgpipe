package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// viewSourceColumns renders the column selection screen
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

// handleSourceColumnsKeys handles key presses on the source columns screen
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
