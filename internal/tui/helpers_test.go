package tui

import (
	"strings"
	"testing"

	"github.com/pgpipe/pgpipe/internal/db"
)

// ── truncate ──────────────────────────────────────────────────────────────

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"over length truncated", "hello world", 8, "hello..."},
		{"empty string", "", 10, ""},
		{"max=3 just ellipsis", "hello", 3, "..."},
		{"unicode safe", "abcdef", 5, "ab..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

// ── repeatChar ────────────────────────────────────────────────────────────

func TestRepeatChar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		c        rune
		n        int
		expected string
	}{
		{"repeat █ 5 times", '█', 5, "█████"},
		{"repeat ░ 3 times", '░', 3, "░░░"},
		{"zero repetitions", 'x', 0, ""},
		{"single char", 'a', 1, "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repeatChar(tt.c, tt.n)
			if result != tt.expected {
				t.Errorf("repeatChar(%q, %d) = %q, want %q", tt.c, tt.n, result, tt.expected)
			}
		})
	}
}

// ── progressBar ──────────────────────────────────────────────────────────

func TestProgressBar(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		percent       float64
		width         int
		containsFull  string
		containsEmpty string
		containsPct   string
	}{
		{"0%", 0.0, 10, "", "░░░░░░░░░░", "0.0%"},
		{"100%", 100.0, 10, "██████████", "", "100.0%"},
		{"50%", 50.0, 10, "█████", "░░░░░", "50.0%"},
		{"over 100 clamped", 150.0, 10, "██████████", "", "150.0%"},
		{"negative clamped to 0", -10.0, 10, "", "░░░░░░░░░░", "-10.0%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := progressBar(tt.percent, tt.width)

			if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
				t.Errorf("progressBar should contain brackets, got %q", result)
			}
			if tt.containsFull != "" && !strings.Contains(result, tt.containsFull) {
				t.Errorf("progressBar(%v, %d) should contain %q, got %q",
					tt.percent, tt.width, tt.containsFull, result)
			}
			if tt.containsEmpty != "" && !strings.Contains(result, tt.containsEmpty) {
				t.Errorf("progressBar(%v, %d) should contain %q, got %q",
					tt.percent, tt.width, tt.containsEmpty, result)
			}
			if !strings.Contains(result, tt.containsPct) {
				t.Errorf("progressBar(%v, %d) should contain %q, got %q",
					tt.percent, tt.width, tt.containsPct, result)
			}
		})
	}
}

// ── filterColumns ────────────────────────────────────────────────────────

func TestFilterColumns(t *testing.T) {
	t.Parallel()

	columns := []db.ColumnInfo{
		{Name: "user_id", DataType: "bigint"},
		{Name: "email", DataType: "varchar"},
		{Name: "created_at", DataType: "timestamp"},
		{Name: "is_active", DataType: "boolean"},
	}

	t.Run("no query clears filter", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: ""},
		}
		m.filterColumns()
		if m.ui.FilteredColumns != nil {
			t.Errorf("FilteredColumns should be nil when no search query, got %v", m.ui.FilteredColumns)
		}
	})

	t.Run("match by name", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: "email"},
		}
		m.filterColumns()
		if len(m.ui.FilteredColumns) != 1 {
			t.Fatalf("Expected 1 match, got %d", len(m.ui.FilteredColumns))
		}
		if m.ui.FilteredColumns[0].Name != "email" {
			t.Errorf("Expected email, got %q", m.ui.FilteredColumns[0].Name)
		}
	})

	t.Run("match by data type", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: "bigint"},
		}
		m.filterColumns()
		if len(m.ui.FilteredColumns) != 1 {
			t.Fatalf("Expected 1 match, got %d", len(m.ui.FilteredColumns))
		}
		if m.ui.FilteredColumns[0].Name != "user_id" {
			t.Errorf("Expected user_id, got %q", m.ui.FilteredColumns[0].Name)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: "EMAIL"},
		}
		m.filterColumns()
		if len(m.ui.FilteredColumns) != 1 {
			t.Fatalf("Expected 1 match for EMAIL, got %d", len(m.ui.FilteredColumns))
		}
	})

	t.Run("no match returns empty", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: "nonexistent_xyz"},
		}
		m.filterColumns()
		if len(m.ui.FilteredColumns) != 0 {
			t.Errorf("Expected no matches, got %d", len(m.ui.FilteredColumns))
		}
	})

	t.Run("partial match", func(t *testing.T) {
		m := Model{
			data: DataCache{MySQLColumns: columns},
			ui:   UIState{SearchQuery: "_at"},
		}
		m.filterColumns()
		// created_at matches
		if len(m.ui.FilteredColumns) != 1 {
			t.Errorf("Expected 1 match for '_at', got %d: %v", len(m.ui.FilteredColumns), m.ui.FilteredColumns)
		}
	})
}

// ── filterTables ─────────────────────────────────────────────────────────

func TestFilterTables(t *testing.T) {
	t.Parallel()

	tables := []db.TableInfo{
		{Name: "users", RowCount: 1000},
		{Name: "orders", RowCount: 5000},
		{Name: "user_profiles", RowCount: 800},
		{Name: "products", RowCount: 200},
	}

	t.Run("no query clears filter", func(t *testing.T) {
		m := Model{ui: UIState{TableSearchQuery: ""}}
		m.filterTables(tables)
		if m.ui.FilteredTables != nil {
			t.Errorf("FilteredTables should be nil when no query, got %v", m.ui.FilteredTables)
		}
	})

	t.Run("match by name prefix", func(t *testing.T) {
		m := Model{ui: UIState{TableSearchQuery: "user"}}
		m.filterTables(tables)
		if len(m.ui.FilteredTables) != 2 {
			t.Fatalf("Expected 2 matches for 'user', got %d: %v", len(m.ui.FilteredTables), m.ui.FilteredTables)
		}
	})

	t.Run("exact match", func(t *testing.T) {
		m := Model{ui: UIState{TableSearchQuery: "orders"}}
		m.filterTables(tables)
		if len(m.ui.FilteredTables) != 1 {
			t.Fatalf("Expected 1 match for 'orders', got %d", len(m.ui.FilteredTables))
		}
		if m.ui.FilteredTables[0].Name != "orders" {
			t.Errorf("Expected orders, got %q", m.ui.FilteredTables[0].Name)
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		m := Model{ui: UIState{TableSearchQuery: "ORDERS"}}
		m.filterTables(tables)
		if len(m.ui.FilteredTables) != 1 {
			t.Fatalf("Expected 1 match for ORDERS, got %d", len(m.ui.FilteredTables))
		}
	})

	t.Run("no match", func(t *testing.T) {
		m := Model{ui: UIState{TableSearchQuery: "nonexistent_table_xyz"}}
		m.filterTables(tables)
		if len(m.ui.FilteredTables) != 0 {
			t.Errorf("Expected no matches, got %d", len(m.ui.FilteredTables))
		}
	})
}
