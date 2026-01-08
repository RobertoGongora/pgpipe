package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// ============================================================================
// Running Screen
// ============================================================================

func (m Model) viewRunning() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration in Progress"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render(fmt.Sprintf("%s → %s", m.selection.SourceTable, m.selection.TargetTable)))
	sb.WriteString("\n\n")

	// Show initialization status if migrator not ready yet
	if m.migration.Migrator == nil {
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
		if m.migration.Migrator != nil {
			m.migration.Migrator.Stop()
		}
		m.screen = ScreenSummary
	case "enter":
		if m.state != nil && m.state.IsComplete() {
			m.screen = ScreenSummary
		}
	}
	return m, nil
}
