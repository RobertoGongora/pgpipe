package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RobertoGongora/pgpipe/internal/tui/styles"
)

// ============================================================================
// Target Table Screen
// ============================================================================

func (m Model) viewTargetTable() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Select Target Table (PostgreSQL)"))
	sb.WriteString("\n\n")

	if len(m.data.PGTables) == 0 {
		sb.WriteString("Loading tables...\n")
		return sb.String()
	}

	// Show search box if in search mode
	if m.ui.TableSearchMode {
		searchBox := fmt.Sprintf("Search: %s_", m.ui.TableSearchQuery)
		sb.WriteString(styles.InputFocused.Render(searchBox))
		sb.WriteString("\n\n")
	}

	// Use filtered tables if searching, otherwise use all tables
	tables := m.data.PGTables
	if m.ui.TableSearchMode && len(m.ui.TableSearchQuery) > 0 {
		tables = m.ui.FilteredTables
	}

	// Show "no results" message if search returned nothing
	if m.ui.TableSearchMode && len(m.ui.TableSearchQuery) > 0 && len(tables) == 0 {
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
	if m.selection.TableCursor >= visibleCount {
		startIdx = m.selection.TableCursor - visibleCount + 1
	}
	endIdx := startIdx + visibleCount
	if endIdx > len(tables) {
		endIdx = len(tables)
	}

	for i := startIdx; i < endIdx; i++ {
		t := tables[i]
		line := fmt.Sprintf("%-40s %s rows", t.Name, styles.FormatNumber(t.RowCount))
		if i == m.selection.TableCursor {
			sb.WriteString(styles.SelectedItem.Render("▸ " + line))
		} else {
			sb.WriteString(styles.ListItem.Render("  " + line))
		}
		sb.WriteString("\n")
	}

	if len(tables) > visibleCount {
		sb.WriteString(fmt.Sprintf("\n(%d/%d tables)", m.selection.TableCursor+1, len(tables)))
	}
	if m.ui.TableSearchMode && len(m.ui.TableSearchQuery) > 0 {
		sb.WriteString(fmt.Sprintf(" (%d matches)", len(tables)))
	}

	sb.WriteString("\n")

	// Different help text depending on mode
	if m.ui.TableSearchMode {
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
	if m.ui.TableSearchMode {
		switch msg.String() {
		case "q":
			m.quitting = true
			return m, tea.Quit
		case "esc":
			// Exit search mode if in search, otherwise go back to previous screen
			if m.ui.TableSearchQuery != "" {
				m.ui.TableSearchMode = false
				m.ui.TableSearchQuery = ""
				m.ui.FilteredTables = nil
				m.selection.TableCursor = 0
			} else {
				m.ui.TableSearchMode = false
				m.screen = ScreenSourceColumns
			}
		case "backspace":
			if len(m.ui.TableSearchQuery) > 0 {
				m.ui.TableSearchQuery = m.ui.TableSearchQuery[:len(m.ui.TableSearchQuery)-1]
				m.filterTables(m.data.PGTables)
			}
		case "up", "k":
			if m.selection.TableCursor > 0 {
				m.selection.TableCursor--
			}
		case "down", "j":
			tables := m.ui.FilteredTables
			if len(tables) > 0 && m.selection.TableCursor < len(tables)-1 {
				m.selection.TableCursor++
			}
		case "enter":
			// Select from filtered results
			if len(m.ui.FilteredTables) > 0 && m.selection.TableCursor < len(m.ui.FilteredTables) {
				m.selection.TargetTable = m.ui.FilteredTables[m.selection.TableCursor].Name
				// Find original index in pgTables
				for i, t := range m.data.PGTables {
					if t.Name == m.selection.TargetTable {
						m.selection.TargetTableIdx = i
						break
					}
				}

				// Save target table selection
				m.config.Migration.Target.Table = m.selection.TargetTable
				m.config.Save()

				m.screen = ScreenMapping
				// Exit search mode
				m.ui.TableSearchMode = false
				m.ui.TableSearchQuery = ""
				m.ui.FilteredTables = nil
				return m, m.loadPGColumns
			}
		default:
			// Add character to search query
			if len(msg.String()) == 1 && msg.String() != "/" {
				m.ui.TableSearchQuery += msg.String()
				m.filterTables(m.data.PGTables)
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
		m.ui.TableSearchMode = true
		m.ui.TableSearchQuery = ""
		m.ui.FilteredTables = nil
		m.selection.TableCursor = 0
	case "up", "k":
		if m.selection.TableCursor > 0 {
			m.selection.TableCursor--
		}
	case "down", "j":
		if m.selection.TableCursor < len(m.data.PGTables)-1 {
			m.selection.TableCursor++
		}
	case "enter":
		if len(m.data.PGTables) > 0 {
			m.selection.TargetTable = m.data.PGTables[m.selection.TableCursor].Name
			m.selection.TargetTableIdx = m.selection.TableCursor

			// Save target table selection
			m.config.Migration.Target.Table = m.selection.TargetTable
			m.config.Save()

			m.screen = ScreenMapping
			return m, m.loadPGColumns
		}
	}
	return m, nil
}
