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
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Table: %s", m.selection.SourceTable)))
	sb.WriteString("\n\n")

	if len(m.data.MySQLColumns) == 0 {
		sb.WriteString("Loading columns...\n")
		return sb.String()
	}

	// Show search box if in search mode
	if m.ui.SearchMode {
		searchBox := fmt.Sprintf("Search: %s_", m.ui.SearchQuery)
		sb.WriteString(styles.InputFocused.Render(searchBox))
		sb.WriteString("\n\n")
	}

	// Find primary key
	var pkName string
	for _, col := range m.data.MySQLColumns {
		if col.IsPrimaryKey {
			pkName = col.Name
			break
		}
	}
	sb.WriteString(styles.Label.Render(fmt.Sprintf("Primary Key: %s (auto-included)\n\n", pkName)))

	// Use filtered columns if searching, otherwise use all columns
	columns := m.data.MySQLColumns
	if m.ui.SearchMode && len(m.ui.SearchQuery) > 0 {
		columns = m.ui.FilteredColumns
	}

	// Show "no results" message if search returned nothing
	if m.ui.SearchMode && len(m.ui.SearchQuery) > 0 && len(columns) == 0 {
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
	startIdx := m.selection.ColumnCursor - visibleCount/2
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
		if m.selection.SelectedColumns[col.Name] {
			checkbox = styles.CheckboxChecked.Render("[✓]")
		}

		line := fmt.Sprintf("%s %-25s %s", checkbox, col.Name, col.DataType)
		if i == m.selection.ColumnCursor {
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
	for _, v := range m.selection.SelectedColumns {
		if v {
			selected++
		}
	}

	sb.WriteString(fmt.Sprintf("\n%d columns selected", selected))
	if m.ui.SearchMode && len(m.ui.SearchQuery) > 0 {
		sb.WriteString(fmt.Sprintf(" (%d matches)\n", len(columns)))
	} else {
		sb.WriteString("\n")
	}

	// Different help text depending on mode
	if m.ui.SearchMode {
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
	if m.ui.SearchMode {
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Exit search mode
			m.ui.SearchMode = false
			m.ui.SearchQuery = ""
			m.ui.FilteredColumns = nil
			m.selection.ColumnCursor = 0
		case "backspace":
			if len(m.ui.SearchQuery) > 0 {
				m.ui.SearchQuery = m.ui.SearchQuery[:len(m.ui.SearchQuery)-1]
				m.filterColumns()
			}
		case "up", "k":
			// Navigate in filtered results
			if m.selection.ColumnCursor > 0 {
				m.selection.ColumnCursor--
			}
		case "down", "j":
			// Navigate in filtered results
			columns := m.ui.FilteredColumns
			if len(columns) > 0 && m.selection.ColumnCursor < len(columns)-1 {
				m.selection.ColumnCursor++
			}
		case " ":
			// Toggle selection in filtered results
			if len(m.ui.FilteredColumns) > 0 && m.selection.ColumnCursor < len(m.ui.FilteredColumns) {
				col := m.ui.FilteredColumns[m.selection.ColumnCursor]
				if !col.IsPrimaryKey {
					m.selection.SelectedColumns[col.Name] = !m.selection.SelectedColumns[col.Name]
				}
			}
		default:
			// Add character to search query (only single chars)
			if len(msg.String()) == 1 && msg.String() != "/" {
				m.ui.SearchQuery += msg.String()
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
		m.ui.SearchMode = true
		m.ui.SearchQuery = ""
		m.ui.FilteredColumns = nil
		m.selection.ColumnCursor = 0
	case "up", "k":
		if m.selection.ColumnCursor > 0 {
			m.selection.ColumnCursor--
		}
	case "down", "j":
		if m.selection.ColumnCursor < len(m.data.MySQLColumns)-1 {
			m.selection.ColumnCursor++
		}
	case " ":
		if len(m.data.MySQLColumns) > 0 {
			col := m.data.MySQLColumns[m.selection.ColumnCursor]
			if !col.IsPrimaryKey {
				m.selection.SelectedColumns[col.Name] = !m.selection.SelectedColumns[col.Name]
			}
		}
	case "a":
		for _, col := range m.data.MySQLColumns {
			if !col.IsPrimaryKey {
				m.selection.SelectedColumns[col.Name] = true
			}
		}
	case "n":
		m.selection.SelectedColumns = make(map[string]bool)
	case "enter":
		// Check at least one column selected
		hasSelection := false
		for _, v := range m.selection.SelectedColumns {
			if v {
				hasSelection = true
				break
			}
		}
		if hasSelection {
			// Build selected columns list
			var selectedCols []string
			for _, col := range m.data.MySQLColumns {
				if m.selection.SelectedColumns[col.Name] && !col.IsPrimaryKey {
					selectedCols = append(selectedCols, col.Name)
				}
			}

			// Get PK
			var pkCol string
			for _, col := range m.data.MySQLColumns {
				if col.IsPrimaryKey {
					pkCol = col.Name
					break
				}
			}

			// Save column selection
			m.config.Migration.Source.Columns = selectedCols
			m.config.Migration.Source.PrimaryKey = pkCol
			m.config.Save()

			m.selection.TableCursor = 0 // Reset for target table selection
			m.screen = ScreenTargetTable
		}
	}
	return m, nil
}
