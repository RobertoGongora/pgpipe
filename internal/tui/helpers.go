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

// filterColumns filters m.data.MySQLColumns based on search query
func (m *Model) filterColumns() {
	if m.ui.SearchQuery == "" {
		m.ui.FilteredColumns = nil
		return
	}

	query := strings.ToLower(m.ui.SearchQuery)
	m.ui.FilteredColumns = nil

	for _, col := range m.data.MySQLColumns {
		if strings.Contains(strings.ToLower(col.Name), query) ||
			strings.Contains(strings.ToLower(col.DataType), query) {
			m.ui.FilteredColumns = append(m.ui.FilteredColumns, col)
		}
	}

	// Reset cursor if out of bounds
	if m.selection.ColumnCursor >= len(m.ui.FilteredColumns) {
		m.selection.ColumnCursor = 0
	}
}

// filterTables filters tables based on search query
func (m *Model) filterTables(tables []db.TableInfo) {
	if m.ui.TableSearchQuery == "" {
		m.ui.FilteredTables = nil
		return
	}

	query := strings.ToLower(m.ui.TableSearchQuery)
	m.ui.FilteredTables = nil

	for _, table := range tables {
		if strings.Contains(strings.ToLower(table.Name), query) {
			m.ui.FilteredTables = append(m.ui.FilteredTables, table)
		}
	}

	// Reset cursor if out of bounds
	if m.selection.TableCursor >= len(m.ui.FilteredTables) {
		m.selection.TableCursor = 0
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

// isIntType checks if a MySQL data type is an integer type
func isIntType(dataType string) bool {
	switch dataType {
	case "tinyint", "smallint", "mediumint", "int", "bigint":
		return true
	}
	return false
}

// isBoolType checks if a PostgreSQL data type is a boolean type
func isBoolType(dataType string) bool {
	switch dataType {
	case "boolean", "bool":
		return true
	}
	return false
}
