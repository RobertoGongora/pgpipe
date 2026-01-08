package styles

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	Primary    = lipgloss.Color("#7C3AED") // Purple
	Secondary  = lipgloss.Color("#06B6D4") // Cyan
	Success    = lipgloss.Color("#10B981") // Green
	Warning    = lipgloss.Color("#F59E0B") // Amber
	Error      = lipgloss.Color("#EF4444") // Red
	Muted      = lipgloss.Color("#6B7280") // Gray
	Background = lipgloss.Color("#1F2937") // Dark gray

	// Base styles
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
		Foreground(Secondary).
		MarginBottom(1)

	// Box styles
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Muted).
		Padding(1, 2)

	FocusedBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(1, 2)

	// List item styles
	ListItem = lipgloss.NewStyle().
		PaddingLeft(2)

	SelectedItem = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(Primary).
		Bold(true)

	// Status styles
	StatusSuccess = lipgloss.NewStyle().
		Foreground(Success)

	StatusError = lipgloss.NewStyle().
		Foreground(Error)

	StatusWarning = lipgloss.NewStyle().
		Foreground(Warning)

	StatusMuted = lipgloss.NewStyle().
		Foreground(Muted)

	// Progress bar
	ProgressFilled = lipgloss.NewStyle().
		Foreground(Success).
		Background(Success)

	ProgressEmpty = lipgloss.NewStyle().
		Foreground(Muted).
		Background(Muted)

	// Help text
	Help = lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(1)

	// Labels
	Label = lipgloss.NewStyle().
		Foreground(Muted)

	Value = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	// Checkboxes
	Checkbox = lipgloss.NewStyle().
		Foreground(Primary)

	CheckboxChecked = lipgloss.NewStyle().
		Foreground(Success)

	// Input
	InputStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Muted).
		Padding(0, 1)

	InputFocused = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(Primary).
		Padding(0, 1)

	// Table
	TableHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(Secondary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(Muted)

	TableCell = lipgloss.NewStyle().
		Padding(0, 1)

	TableCellSelected = lipgloss.NewStyle().
		Padding(0, 1).
		Background(Primary).
		Foreground(lipgloss.Color("#FFFFFF"))
)

// FormatNumber formats a number with commas
func FormatNumber(n int64) string {
	if n < 0 {
		return "-" + FormatNumber(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return FormatNumber(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}
