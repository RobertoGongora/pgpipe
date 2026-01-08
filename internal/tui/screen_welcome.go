package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/migration"
	"github.com/pgpipe/pgpipe/internal/tui/styles"
)

// viewWelcome renders the welcome screen
func (m Model) viewWelcome() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("pgpipe"))
	sb.WriteString("\n")
	sb.WriteString(styles.Subtitle.Render("MySQL to PostgreSQL Migration Tool"))
	sb.WriteString("\n\n")

	// Check what we have saved
	hasConfig := m.config.Migration.Source.Table != ""
	hasState := m.ui.HasExistingState && m.state != nil

	if hasState {
		// Active migration found - show progress
		sb.WriteString(styles.Box.Render(fmt.Sprintf(
			"Existing migration found!\n\n"+
				"Source: %s\n"+
				"Progress: %s / %s (%.1f%%)\n"+
				"Last run: %s",
			m.state.Source.Table,
			styles.FormatNumber(m.state.Progress.ProcessedRows),
			styles.FormatNumber(m.state.Source.TotalRows),
			m.state.ProgressPercent(),
			m.state.LastRun.EndedAt.Format("2006-01-02 15:04"),
		)))
		sb.WriteString("\n\n")

		options := []string{"Resume migration", "Start new migration"}
		for i, opt := range options {
			if i == m.ui.ResumeChoice {
				sb.WriteString(styles.SelectedItem.Render("▸ " + opt))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + opt))
			}
			sb.WriteString("\n")
		}
	} else if hasConfig {
		// Saved config found but no active migration
		sb.WriteString(styles.Box.Render(fmt.Sprintf(
			"Saved configuration found!\n\n"+
				"Source: %s → %s\n"+
				"Columns: %d mapped\n"+
				"Batch size: %d",
			m.config.Migration.Source.Table,
			m.config.Migration.Target.Table,
			len(m.config.Migration.Mapping),
			m.config.Migration.Settings.BatchSize,
		)))
		sb.WriteString("\n\n")

		options := []string{"Use saved configuration", "Start fresh"}
		for i, opt := range options {
			if i == m.ui.ResumeChoice {
				sb.WriteString(styles.SelectedItem.Render("▸ " + opt))
			} else {
				sb.WriteString(styles.ListItem.Render("  " + opt))
			}
			sb.WriteString("\n")
		}
	} else {
		// No config or state
		sb.WriteString("No existing configuration found.\n")
		sb.WriteString("Press Enter to start a new migration.\n")
	}

	sb.WriteString("\n")
	sb.WriteString(renderHelp(
		helpItem{Key: "↑/↓", Description: "Navigate"},
		helpItem{Key: "Enter", Description: "Select"},
		helpItem{Key: "q", Description: "Quit"},
	))

	return sb.String()
}

// handleWelcomeKeys handles key presses on the welcome screen
func (m Model) handleWelcomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	hasConfig := m.config.Migration.Source.Table != ""
	hasState := m.ui.HasExistingState && m.state != nil

	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.ui.ResumeChoice > 0 {
			m.ui.ResumeChoice--
		}
	case "down", "j":
		if (hasState || hasConfig) && m.ui.ResumeChoice < 1 {
			m.ui.ResumeChoice++
		}
	case "enter":
		if hasState && m.ui.ResumeChoice == 0 {
			// Resume existing migration - jump to settings
			// Config and state are already loaded in NewModel()
			m.screen = ScreenSettings
			return m, m.connectDatabases
		} else if hasConfig && m.ui.ResumeChoice == 0 {
			// Use saved config - config is already loaded in NewModel()
			// Jump to settings since everything is pre-configured
			m.screen = ScreenSettings
			return m, m.connectDatabases
		} else if (hasState || hasConfig) && m.ui.ResumeChoice == 1 {
			// Start fresh - delete state and config
			migration.DeleteState()
			m.state = nil
			m.ui.HasExistingState = false

			// Reset config to defaults
			m.config = config.NewDefaultConfig()
			m.config.Save()

			// Reset all selections
			m.selection.SelectedColumns = make(map[string]bool)
			m.selection.SourceTable = ""
			m.selection.TargetTable = ""
			m.selection.ColumnMappings = nil
			m.settings.BatchSize = 5000
			m.settings.BatchLimit = 100

			m.screen = ScreenConnections
			return m, m.connectDatabases
		} else {
			// No existing state or config - start new
			m.screen = ScreenConnections
			return m, m.connectDatabases
		}
	}
	return m, nil
}
