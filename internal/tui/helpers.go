package tui

import (
	"fmt"
	"strings"

	"github.com/pgpipe/pgpipe/internal/db"
)

// truncate shortens a string to max length with ellipsis
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// progressBar creates a visual progress bar
func progressBar(percent float64, width int) string {
	filled := int(float64(width) * percent / 100)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s] %.1f%%",
		repeatChar('█', filled),
		repeatChar('░', empty),
		percent)
}

// repeatChar creates a string of repeated characters
func repeatChar(c rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

// filterColumns filters m.mysqlColumns based on search query
func (m *Model) filterColumns() {
	if m.searchQuery == "" {
		m.filteredColumns = nil
		return
	}

	query := strings.ToLower(m.searchQuery)
	m.filteredColumns = nil

	for _, col := range m.mysqlColumns {
		if strings.Contains(strings.ToLower(col.Name), query) ||
			strings.Contains(strings.ToLower(col.DataType), query) {
			m.filteredColumns = append(m.filteredColumns, col)
		}
	}

	// Reset cursor if out of bounds
	if m.columnCursor >= len(m.filteredColumns) {
		m.columnCursor = 0
	}
}

// filterTables filters tables based on search query
func (m *Model) filterTables(tables []db.TableInfo) {
	if m.tableSearchQuery == "" {
		m.filteredTables = nil
		return
	}

	query := strings.ToLower(m.tableSearchQuery)
	m.filteredTables = nil

	for _, table := range tables {
		if strings.Contains(strings.ToLower(table.Name), query) {
			m.filteredTables = append(m.filteredTables, table)
		}
	}

	// Reset cursor if out of bounds
	if m.tableCursor >= len(m.filteredTables) {
		m.tableCursor = 0
	}
}

// isTextType checks if a MySQL data type is a text type
func isTextType(dataType string) bool {
	switch dataType {
	case "text", "mediumtext", "longtext", "varchar", "char":
		return true
	}
	return false
}

// isJSONType checks if a PostgreSQL data type is a JSON type
func isJSONType(dataType string) bool {
	switch dataType {
	case "json", "jsonb":
		return true
	}
	return false
}
