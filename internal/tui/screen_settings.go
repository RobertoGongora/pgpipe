package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pgpipe/pgpipe/internal/migration"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
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
