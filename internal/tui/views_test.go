package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
	"github.com/pgpipe/pgpipe/internal/migration"
)

// newTestModel returns a minimal Model suitable for rendering tests.
func newTestModel() Model {
	return Model{
		screen: ScreenWelcome,
		config: config.NewDefaultConfig(),
		selection: SelectionState{
			SelectedColumns: make(map[string]bool),
		},
		settings: SettingsState{
			BatchSize:  5000,
			RunMode:    migration.RunModeBatches,
			BatchLimit: 100,
		},
	}
}

// ── renderHelp ────────────────────────────────────────────────────────────

func TestRenderHelp(t *testing.T) {
	t.Parallel()

	t.Run("empty renders empty string", func(t *testing.T) {
		result := renderHelp()
		if result != "" {
			t.Errorf("renderHelp() with no args = %q, want %q", result, "")
		}
	})

	t.Run("single item contains key and description", func(t *testing.T) {
		result := renderHelp(helpItem{Key: "q", Description: "Quit"})
		if !strings.Contains(result, "q") {
			t.Errorf("renderHelp() should contain key 'q', got %q", result)
		}
		if !strings.Contains(result, "Quit") {
			t.Errorf("renderHelp() should contain 'Quit', got %q", result)
		}
	})

	t.Run("multiple items all present", func(t *testing.T) {
		result := renderHelp(
			helpItem{Key: "↑/↓", Description: "Navigate"},
			helpItem{Key: "Enter", Description: "Select"},
			helpItem{Key: "Esc", Description: "Back"},
		)
		for _, want := range []string{"↑/↓", "Navigate", "Enter", "Select", "Esc", "Back"} {
			if !strings.Contains(result, want) {
				t.Errorf("renderHelp() should contain %q, got %q", want, result)
			}
		}
	})
}

// ── viewWelcome ───────────────────────────────────────────────────────────

func TestViewWelcomeNoConfig(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	// No config, no state
	view := m.viewWelcome()

	if !strings.Contains(view, "pgpipe") {
		t.Errorf("viewWelcome() should contain 'pgpipe', got:\n%s", view)
	}
	if !strings.Contains(view, "No existing configuration") {
		t.Errorf("viewWelcome() should say no config found, got:\n%s", view)
	}
}

func TestViewWelcomeWithConfig(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.config.Migration.Source.Table = "users"
	m.config.Migration.Target.Table = "pg_users"
	m.config.Migration.Settings.BatchSize = 1000

	view := m.viewWelcome()

	if !strings.Contains(view, "Saved configuration found") {
		t.Errorf("viewWelcome() should show saved config, got:\n%s", view)
	}
	if !strings.Contains(view, "users") {
		t.Errorf("viewWelcome() should show source table name, got:\n%s", view)
	}
}

func TestViewWelcomeWithState(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.ui.HasExistingState = true
	m.state = &migration.State{
		Source: migration.SourceState{
			Table:     "orders",
			TotalRows: 50000,
		},
		Progress: migration.ProgressState{
			ProcessedRows: 25000,
		},
		LastRun: migration.LastRunState{
			EndedAt: time.Now(),
		},
	}

	view := m.viewWelcome()

	if !strings.Contains(view, "Existing migration found") {
		t.Errorf("viewWelcome() should show existing migration, got:\n%s", view)
	}
	if !strings.Contains(view, "orders") {
		t.Errorf("viewWelcome() should show source table, got:\n%s", view)
	}
	if !strings.Contains(view, "Resume migration") {
		t.Errorf("viewWelcome() should show resume option, got:\n%s", view)
	}
}

// ── viewSummary ───────────────────────────────────────────────────────────

func TestViewSummaryNoState(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.screen = ScreenSummary
	// No state set

	view := m.viewSummary()

	if !strings.Contains(view, "Migration Summary") {
		t.Errorf("viewSummary() should contain title, got:\n%s", view)
	}
	if !strings.Contains(view, "No migration data") {
		t.Errorf("viewSummary() should say no data, got:\n%s", view)
	}
}

func TestViewSummaryComplete(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.screen = ScreenSummary
	m.state = &migration.State{
		Source: migration.SourceState{
			TotalRows: 10000,
			MaxID:     10000,
		},
		Progress: migration.ProgressState{
			LastCursor:    10000,
			ProcessedRows: 10000,
			ImportedRows:  9990,
			SkippedRows:   10,
		},
		LastRun: migration.LastRunState{
			DurationSeconds:  12.5,
			BatchesCompleted: 10,
			RowsThisRun:      10000,
		},
	}

	view := m.viewSummary()

	if !strings.Contains(view, "Migration Summary") {
		t.Errorf("viewSummary() should contain title, got:\n%s", view)
	}
	if !strings.Contains(view, "Migration Complete") {
		t.Errorf("viewSummary() should show complete status, got:\n%s", view)
	}
	// Check for row numbers (formatted)
	if !strings.Contains(view, "10,000") {
		t.Errorf("viewSummary() should show formatted row count, got:\n%s", view)
	}
}

func TestViewSummaryPaused(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.screen = ScreenSummary
	m.state = &migration.State{
		Source: migration.SourceState{
			TotalRows: 100000,
			MaxID:     100000,
		},
		Progress: migration.ProgressState{
			LastCursor:    30000,
			ProcessedRows: 30000,
			ImportedRows:  30000,
			SkippedRows:   0,
		},
		LastRun: migration.LastRunState{
			DurationSeconds:  45.0,
			BatchesCompleted: 30,
			RowsThisRun:      30000,
		},
	}

	view := m.viewSummary()

	if !strings.Contains(view, "Migration Paused") {
		t.Errorf("viewSummary() should show paused status, got:\n%s", view)
	}
	if !strings.Contains(view, "pgpipe") {
		t.Errorf("viewSummary() should mention pgpipe for resumption, got:\n%s", view)
	}
}

// ── View() routing ────────────────────────────────────────────────────────

func TestViewRouting(t *testing.T) {
	t.Parallel()

	screens := []struct {
		screen      Screen
		mustContain string
	}{
		{ScreenWelcome, "pgpipe"},
		{ScreenSummary, "Migration Summary"},
	}

	for _, tt := range screens {
		t.Run("", func(t *testing.T) {
			m := newTestModel()
			m.screen = tt.screen
			view := m.View()
			if !strings.Contains(view, tt.mustContain) {
				t.Errorf("View() for screen %d should contain %q, got:\n%s",
					tt.screen, tt.mustContain, view)
			}
		})
	}
}

func TestViewQuitting(t *testing.T) {
	t.Parallel()

	m := newTestModel()
	m.quitting = true

	view := m.View()
	if !strings.Contains(view, "Goodbye") {
		t.Errorf("View() when quitting should say Goodbye, got %q", view)
	}
}

// ── generateAutoMappings ─────────────────────────────────────────────────

func TestGenerateAutoMappings(t *testing.T) {
	t.Parallel()

	t.Run("matching columns are mapped", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "id", DataType: "bigint", IsPrimaryKey: true},
			{Name: "email", DataType: "varchar"},
			{Name: "name", DataType: "varchar"},
			{Name: "mysql_only", DataType: "text"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "id", DataType: "bigint"},
			{Name: "email", DataType: "varchar"},
			{Name: "name", DataType: "varchar"},
		}
		m.selection.SelectedColumns = map[string]bool{
			"email":      true,
			"name":       true,
			"mysql_only": true,
		}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) == 0 {
			t.Fatal("generateAutoMappings() should create mappings")
		}

		// email and name should be mapped; mysql_only has no PG match
		mappedTargets := make(map[string]string)
		for _, cm := range m.selection.ColumnMappings {
			mappedTargets[cm.Source] = cm.Target
		}

		if mappedTargets["email"] != "email" {
			t.Errorf("email should be mapped to email, got %q", mappedTargets["email"])
		}
		if mappedTargets["name"] != "name" {
			t.Errorf("name should be mapped to name, got %q", mappedTargets["name"])
		}
		if mappedTargets["mysql_only"] != "" {
			t.Errorf("mysql_only should not be mapped (no PG match), got %q", mappedTargets["mysql_only"])
		}
	})

	t.Run("unselected columns are skipped", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "id", DataType: "bigint"},
			{Name: "email", DataType: "varchar"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "id", DataType: "bigint"},
			{Name: "email", DataType: "varchar"},
		}
		m.selection.SelectedColumns = map[string]bool{
			"email": true,
			// "id" not selected
		}

		m.generateAutoMappings()

		for _, cm := range m.selection.ColumnMappings {
			if cm.Source == "id" {
				t.Error("generateAutoMappings() should not map unselected column 'id'")
			}
		}
	})

	t.Run("text_to_jsonb transform auto-detected", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "data", DataType: "longtext"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "data", DataType: "jsonb"},
		}
		m.selection.SelectedColumns = map[string]bool{"data": true}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(m.selection.ColumnMappings))
		}
		if m.selection.ColumnMappings[0].Transform != "text_to_jsonb" {
			t.Errorf("Transform = %q, want text_to_jsonb", m.selection.ColumnMappings[0].Transform)
		}
	})

	t.Run("int_to_bool transform auto-detected", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "active", DataType: "tinyint"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "active", DataType: "boolean"},
		}
		m.selection.SelectedColumns = map[string]bool{"active": true}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(m.selection.ColumnMappings))
		}
		if m.selection.ColumnMappings[0].Transform != "int_to_bool" {
			t.Errorf("Transform = %q, want int_to_bool", m.selection.ColumnMappings[0].Transform)
		}
	})

	t.Run("string_to_uuid transform auto-detected", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "user_id", DataType: "varchar"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "user_id", DataType: "uuid"},
		}
		m.selection.SelectedColumns = map[string]bool{"user_id": true}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(m.selection.ColumnMappings))
		}
		if m.selection.ColumnMappings[0].Transform != "string_to_uuid" {
			t.Errorf("Transform = %q, want string_to_uuid", m.selection.ColumnMappings[0].Transform)
		}
	})

	t.Run("no transform for same type", func(t *testing.T) {
		m := newTestModel()
		m.data.MySQLColumns = []db.ColumnInfo{
			{Name: "name", DataType: "varchar"},
		}
		m.data.PGColumns = []db.ColumnInfo{
			{Name: "name", DataType: "varchar"},
		}
		m.selection.SelectedColumns = map[string]bool{"name": true}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) != 1 {
			t.Fatalf("Expected 1 mapping, got %d", len(m.selection.ColumnMappings))
		}
		if m.selection.ColumnMappings[0].Transform != "" {
			t.Errorf("Transform = %q, want empty (no transform)", m.selection.ColumnMappings[0].Transform)
		}
	})

	t.Run("clears existing mappings", func(t *testing.T) {
		m := newTestModel()
		m.selection.ColumnMappings = []config.ColumnMapping{
			{Source: "old", Target: "mapping"},
		}
		m.data.MySQLColumns = []db.ColumnInfo{}
		m.data.PGColumns = []db.ColumnInfo{}
		m.selection.SelectedColumns = map[string]bool{}

		m.generateAutoMappings()

		if len(m.selection.ColumnMappings) != 0 {
			t.Errorf("generateAutoMappings() should clear old mappings, got %v", m.selection.ColumnMappings)
		}
	})
}
