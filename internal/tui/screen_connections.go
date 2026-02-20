package tui

import (
	"fmt"
	"strings"

	"github.com/RobertoGongora/pgpipe/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
)

// viewConnections renders the database connections screen
func (m Model) viewConnections() string {
	var sb strings.Builder

	sb.WriteString(styles.Title.Render("Database Connections"))
	sb.WriteString("\n\n")

	// MySQL status
	mysqlStatus := "⏳ Connecting..."
	mysqlStyle := styles.StatusMuted
	if m.conn.MySQLConnected {
		mysqlStatus = "✓ Connected"
		mysqlStyle = styles.StatusSuccess
	} else if m.conn.MySQLError != "" {
		mysqlStatus = "✗ " + truncate(m.conn.MySQLError, 50)
		mysqlStyle = styles.StatusError
	}

	sb.WriteString(styles.Box.Render(fmt.Sprintf(
		"MySQL\n"+
			"├─ Host: %s:%d\n"+
			"├─ Database: %s\n"+
			"└─ Status: %s",
		m.config.MySQL.Host,
		m.config.MySQL.Port,
		m.config.MySQL.Database,
		mysqlStyle.Render(mysqlStatus),
	)))
	sb.WriteString("\n\n")

	// PostgreSQL status
	pgStatus := "⏳ Connecting..."
	pgStyle := styles.StatusMuted
	if m.conn.PGConnected {
		pgStatus = "✓ Connected"
		pgStyle = styles.StatusSuccess
	} else if m.conn.PGError != "" {
		pgStatus = "✗ " + truncate(m.conn.PGError, 50)
		pgStyle = styles.StatusError
	}

	sb.WriteString(styles.Box.Render(fmt.Sprintf(
		"PostgreSQL\n"+
			"├─ Host: %s:%d\n"+
			"├─ Database: %s\n"+
			"└─ Status: %s",
		m.config.PostgreSQL.Host,
		m.config.PostgreSQL.Port,
		m.config.PostgreSQL.Database,
		pgStyle.Render(pgStatus),
	)))
	sb.WriteString("\n\n")

	if m.conn.MySQLConnected && m.conn.PGConnected {
		sb.WriteString(renderHelp(
			helpItem{Key: "Enter", Description: "Continue"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else if m.conn.MySQLError != "" || m.conn.PGError != "" {
		sb.WriteString(renderHelp(
			helpItem{Key: "r", Description: "Retry"},
			helpItem{Key: "q", Description: "Quit"},
		))
	} else {
		sb.WriteString(styles.Help.Render("Connecting..."))
	}

	return sb.String()
}

// handleConnectionsKeys handles key presses on the connections screen
func (m Model) handleConnectionsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "r":
		return m, m.connectDatabases
	case "enter":
		if m.conn.MySQLConnected && m.conn.PGConnected {
			m.screen = ScreenSourceTable
		}
	}
	return m, nil
}
