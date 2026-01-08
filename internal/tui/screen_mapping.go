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
