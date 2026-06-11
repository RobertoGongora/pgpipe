package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/RobertoGongora/pgpipe/internal/tui/styles"
)

// ============================================================================
// Summary Screen
// ============================================================================

func (m Model) viewSummary() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Migration Summary"))
	sb.WriteString("\n\n")

	if m.state == nil {
		sb.WriteString("No migration data available.\n")
		sb.WriteString(renderHelp(
			helpItem{Key: "Enter", Description: "Exit"},
		))
		return sb.String()
	}

	// Status. A run that reached the end of the table but skipped rows is NOT a
	// clean load — surface that in the headline rather than a green checkmark.
	switch {
	case !m.state.IsComplete():
		sb.WriteString(styles.StatusWarning.Render("◐ Migration Paused"))
	case m.state.Progress.SkippedRows > 0:
		sb.WriteString(styles.StatusWarning.Render("⚠ Migration Complete — with skipped rows (load incomplete)"))
	default:
		sb.WriteString(styles.StatusSuccess.Render("✓ Migration Complete!"))
	}
	sb.WriteString("\n\n")

	// This run stats
	thisRun := fmt.Sprintf(
		"This Run\n"+
			"Duration:    %.1f seconds\n"+
			"Batches:     %d\n"+
			"Rows:        %s",
		m.state.LastRun.DurationSeconds,
		m.state.LastRun.BatchesCompleted,
		styles.FormatNumber(m.state.LastRun.RowsThisRun),
	)
	sb.WriteString(styles.Box.Render(thisRun))
	sb.WriteString("\n\n")

	// Overall progress
	overall := fmt.Sprintf(
		"Overall Progress\n"+
			"Progress:    %s / %s (%.1f%%)\n"+
			"Imported:    %s\n"+
			"Skipped:     %s\n"+
			"Remaining:   %s rows",
		styles.FormatNumber(m.state.Progress.ProcessedRows),
		styles.FormatNumber(m.state.Source.TotalRows),
		m.state.ProgressPercent(),
		styles.FormatNumber(m.state.Progress.ImportedRows),
		styles.FormatNumber(m.state.Progress.SkippedRows),
		styles.FormatNumber(m.state.RemainingRows()),
	)
	sb.WriteString(styles.Box.Render(overall))
	sb.WriteString("\n\n")

	// Error log
	if m.state.Progress.SkippedRows > 0 {
		errorInfo := fmt.Sprintf(
			"Errors\n"+
				"%d rows skipped\n"+
				"Log: %s",
			m.state.Progress.SkippedRows,
			m.state.Session.ErrorLog,
		)
		sb.WriteString(lipgloss.NewStyle().Foreground(styles.Warning).Render(styles.Box.Render(errorInfo)))
		sb.WriteString("\n\n")
	}

	if !m.state.IsComplete() {
		sb.WriteString("Run `pgpipe` again to continue migration.\n\n")
	}

	sb.WriteString(renderHelp(
		helpItem{Key: "Enter", Description: "Exit"},
	))

	return sb.String()
}

func (m Model) handleSummaryKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "enter":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}
