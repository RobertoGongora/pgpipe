package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/RobertoGongora/pgpipe/internal/migration"
	"github.com/RobertoGongora/pgpipe/internal/tui/styles"
)

// ============================================================================
// Settings Screen
// ============================================================================

func (m Model) viewSettings() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration Settings"))
	sb.WriteString("\n\n")

	// Batch size
	var batchLine string
	if m.settings.EditingBatchSize {
		batchLine = fmt.Sprintf("Batch Size: [%s_]", m.settings.InputBuffer)
	} else {
		batchLine = fmt.Sprintf("Batch Size: %d", m.settings.BatchSize)
	}
	if m.settings.SettingsCursor == 0 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + batchLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + batchLine))
	}
	sb.WriteString("\n\n")

	// Run mode
	sb.WriteString("Run Mode:\n")

	continuousLine := "○ Continuous - Run until complete"
	if m.settings.RunMode == migration.RunModeContinuous {
		continuousLine = "● Continuous - Run until complete"
	}
	if m.settings.SettingsCursor == 1 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + continuousLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + continuousLine))
	}
	sb.WriteString("\n")

	var batchesLine string
	if m.settings.EditingBatchLimit {
		batchesLine = fmt.Sprintf("● Run [%s_] batches then stop", m.settings.InputBuffer)
	} else {
		batchesLine = fmt.Sprintf("○ Run %d batches then stop", m.settings.BatchLimit)
	}
	if m.settings.RunMode == migration.RunModeBatches {
		if !m.settings.EditingBatchLimit {
			batchesLine = fmt.Sprintf("● Run %d batches then stop", m.settings.BatchLimit)
		}
	}
	if m.settings.SettingsCursor == 2 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + batchesLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + batchesLine))
	}
	sb.WriteString("\n\n")

	// Summary
	if len(m.data.MySQLTables) > 0 {
		totalRows := m.data.MySQLTables[m.selection.SourceTableIdx].RowCount
		estimatedBatches := (totalRows + int64(m.settings.BatchSize) - 1) / int64(m.settings.BatchSize)

		var willProcess string
		if m.settings.RunMode == migration.RunModeContinuous {
			willProcess = fmt.Sprintf("%s rows", styles.FormatNumber(totalRows))
		} else {
			rowsThisRun := int64(m.settings.BatchLimit) * int64(m.settings.BatchSize)
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
	if m.settings.SettingsCursor == 3 {
		sb.WriteString(styles.SelectedItem.Render("▸ " + startLine))
	} else {
		sb.WriteString(styles.ListItem.Render("  " + startLine))
	}

	sb.WriteString("\n\n")

	// Different help text depending on mode
	if m.settings.EditingBatchSize || m.settings.EditingBatchLimit {
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
	if m.settings.EditingBatchSize {
		switch msg.String() {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.settings.InputBuffer += msg.String()
		case "backspace":
			if len(m.settings.InputBuffer) > 0 {
				m.settings.InputBuffer = m.settings.InputBuffer[:len(m.settings.InputBuffer)-1]
			}
		case "enter":
			// Apply value with validation
			if val, err := strconv.Atoi(m.settings.InputBuffer); err == nil {
				if val < MinBatchSize {
					val = MinBatchSize
				}
				if val > MaxBatchSize {
					val = MaxBatchSize
				}
				m.settings.BatchSize = val
			}
			m.settings.EditingBatchSize = false
			m.settings.InputBuffer = ""
		case "esc":
			// Cancel editing
			m.settings.EditingBatchSize = false
			m.settings.InputBuffer = ""
		case "q":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	// Handle batch limit editing mode
	if m.settings.EditingBatchLimit {
		switch msg.String() {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.settings.InputBuffer += msg.String()
		case "backspace":
			if len(m.settings.InputBuffer) > 0 {
				m.settings.InputBuffer = m.settings.InputBuffer[:len(m.settings.InputBuffer)-1]
			}
		case "enter":
			// Apply value with validation
			if val, err := strconv.Atoi(m.settings.InputBuffer); err == nil {
				if val < MinBatchLimit {
					val = MinBatchLimit
				}
				if val > MaxBatchLimit {
					val = MaxBatchLimit
				}
				m.settings.BatchLimit = val
			}
			m.settings.EditingBatchLimit = false
			m.settings.InputBuffer = ""
		case "esc":
			// Cancel editing
			m.settings.EditingBatchLimit = false
			m.settings.InputBuffer = ""
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
		if m.settings.SettingsCursor > 0 {
			m.settings.SettingsCursor--
		}
	case "down", "j":
		if m.settings.SettingsCursor < 3 { // Now 3 because we added "Start Migration"
			m.settings.SettingsCursor++
		}
	case "left", "h":
		switch m.settings.SettingsCursor {
		case 0: // Batch size
			if m.settings.BatchSize > 100 {
				m.settings.BatchSize -= 100
			} else if m.settings.BatchSize > 1 {
				m.settings.BatchSize = 1 // Jump to minimum
			}
		case 2: // Batch limit
			if m.settings.BatchLimit > 10 {
				m.settings.BatchLimit -= 10
			} else if m.settings.BatchLimit > 1 {
				m.settings.BatchLimit = 1 // Jump to minimum
			}
		}
	case "right", "l":
		switch m.settings.SettingsCursor {
		case 0: // Batch size
			if m.settings.BatchSize < 50000 {
				m.settings.BatchSize += 100
			}
		case 2: // Batch limit
			if m.settings.BatchLimit < 10000 {
				m.settings.BatchLimit += 10
			}
		}
	case "enter":
		if m.settings.SettingsCursor == 0 {
			// Edit batch size
			m.settings.EditingBatchSize = true
			m.settings.InputBuffer = fmt.Sprintf("%d", m.settings.BatchSize)
		} else if m.settings.SettingsCursor == 1 {
			// Select continuous mode
			m.settings.RunMode = migration.RunModeContinuous
		} else if m.settings.SettingsCursor == 2 {
			// Edit batch limit or select batch mode
			m.settings.RunMode = migration.RunModeBatches
			m.settings.EditingBatchLimit = true
			m.settings.InputBuffer = fmt.Sprintf("%d", m.settings.BatchLimit)
		}
	case "s":
		if !m.settings.EditingBatchSize && !m.settings.EditingBatchLimit {
			// Save final settings before starting
			m.config.Migration.Settings.BatchSize = m.settings.BatchSize
			m.config.Save()

			// Start migration
			m.screen = ScreenRunning
			return m, m.startMigration()
		}
	case " ":
		// Toggle run mode with space
		if m.settings.SettingsCursor == 1 {
			m.settings.RunMode = migration.RunModeContinuous
		} else if m.settings.SettingsCursor == 2 {
			m.settings.RunMode = migration.RunModeBatches
		}
	}
	return m, nil
}
