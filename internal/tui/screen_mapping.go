package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// ============================================================================
// Column Mapping Screen
// ============================================================================

func (m Model) viewMapping() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Column Mapping"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("%s → %s", m.selection.SourceTable, m.selection.TargetTable)))
	sb.WriteString("\n\n")

	if len(m.selection.ColumnMappings) == 0 {
		sb.WriteString("Generating mappings...\n")
	} else {
		// Header
		header := fmt.Sprintf("%-25s %-25s %-15s", "Source", "Target", "Transform")
		sb.WriteString(styles.TableHeader.Render(header))
		sb.WriteString("\n")

		for i, mapping := range m.selection.ColumnMappings {
			target := mapping.Target
			if target == "" {
				target = "(skip)"
			}
			transform := mapping.Transform
			if transform == "" {
				transform = "-"
			}

			line := fmt.Sprintf("%-25s %-25s %-15s", mapping.Source, target, transform)
			if i == m.selection.MappingCursor {
				sb.WriteString(styles.SelectedItem.Render("▸ " + line))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + line))
			}
			sb.WriteString("\n")
		}
	}

	// Show warnings
	var warnings []string
	for _, mapping := range m.selection.ColumnMappings {
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
		if m.selection.MappingCursor > 0 {
			m.selection.MappingCursor--
		}
	case "down", "j":
		if m.selection.MappingCursor < len(m.selection.ColumnMappings)-1 {
			m.selection.MappingCursor++
		}
	case "enter":
		// Enter edit mode for the selected mapping
		m.ui.EditingMapping = true
		m.ui.EditTargetCursor = 0
		// Set available targets from pgColumns
		m.ui.AvailableTargets = m.data.PGColumns

		// Pre-select current target if one is set
		if m.selection.MappingCursor < len(m.selection.ColumnMappings) {
			currentTarget := m.selection.ColumnMappings[m.selection.MappingCursor].Target
			if currentTarget != "" {
				// Find the index of the current target
				for i, col := range m.ui.AvailableTargets {
					if col.Name == currentTarget {
						m.ui.EditTargetCursor = i + 1 // +1 because 0 is skip option
						break
					}
				}
			}
		}
	case "c":
		// Save mappings before continuing
		m.config.Migration.Mapping = m.selection.ColumnMappings
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

	if m.selection.MappingCursor >= len(m.selection.ColumnMappings) {
		return "Invalid mapping selection"
	}

	currentMapping := m.selection.ColumnMappings[m.selection.MappingCursor]

	sb.WriteString(styles.Title.Render("Edit Column Mapping"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("Source: %s (%s)",
		currentMapping.Source,
		m.getSourceColumnType(currentMapping.Source))))
	sb.WriteString("\n\n")

	sb.WriteString("Select target column:\n\n")

	// Calculate scrolling viewport for target columns
	// +1 for skip option at index 0
	totalOptions := len(m.ui.AvailableTargets) + 1
	visibleCount := MaxVisibleTargets

	startIdx := m.ui.EditTargetCursor - visibleCount/2
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
			col := m.ui.AvailableTargets[i-1]
			line = fmt.Sprintf("%-30s %s", col.Name, col.DataType)

			// Show if this column is already mapped from another source
			mappedFrom := m.getSourceMappedTo(col.Name)
			if mappedFrom != "" && mappedFrom != currentMapping.Source {
				line += fmt.Sprintf(" (mapped from: %s)", mappedFrom)
			}
		}

		if i == m.ui.EditTargetCursor {
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
	maxCursor := len(m.ui.AvailableTargets) // +1 for skip at 0, but 0-indexed so it works out

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		// Cancel editing
		m.ui.EditingMapping = false
		m.ui.EditTargetCursor = 0
	case "up", "k":
		if m.ui.EditTargetCursor > 0 {
			m.ui.EditTargetCursor--
		}
	case "down", "j":
		if m.ui.EditTargetCursor < maxCursor {
			m.ui.EditTargetCursor++
		}
	case "enter":
		// Apply the selection
		if m.ui.EditTargetCursor == 0 {
			// Skip option selected
			m.selection.ColumnMappings[m.selection.MappingCursor].Target = ""
			m.selection.ColumnMappings[m.selection.MappingCursor].Transform = ""
		} else {
			// Target column selected
			selectedCol := m.ui.AvailableTargets[m.ui.EditTargetCursor-1]
			m.selection.ColumnMappings[m.selection.MappingCursor].Target = selectedCol.Name

			// Auto-detect transform if TEXT->JSONB
			sourceCol := m.getSourceColumn(m.selection.ColumnMappings[m.selection.MappingCursor].Source)
			if sourceCol != nil && isTextType(sourceCol.DataType) && isJSONType(selectedCol.DataType) {
				m.selection.ColumnMappings[m.selection.MappingCursor].Transform = "text_to_jsonb"
			} else {
				m.selection.ColumnMappings[m.selection.MappingCursor].Transform = ""
			}
		}

		// Save updated mappings
		m.config.Migration.Mapping = m.selection.ColumnMappings
		m.config.Save()

		// Exit edit mode
		m.ui.EditingMapping = false
		m.ui.EditTargetCursor = 0
	}
	return m, nil
}

// Helper methods for mapping editor

func (m Model) getSourceColumnType(colName string) string {
	for _, col := range m.data.MySQLColumns {
		if col.Name == colName {
			return col.DataType
		}
	}
	return "unknown"
}

func (m Model) getSourceColumn(colName string) *db.ColumnInfo {
	for _, col := range m.data.MySQLColumns {
		if col.Name == colName {
			return &col
		}
	}
	return nil
}

func (m Model) getSourceMappedTo(targetColName string) string {
	for _, mapping := range m.selection.ColumnMappings {
		if mapping.Target == targetColName {
			return mapping.Source
		}
	}
	return ""
}
