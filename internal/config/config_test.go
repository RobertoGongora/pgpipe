package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Pure-logic tests (no I/O) ──────────────────────────────────────────────

func TestMySQLDSN(t *testing.T) {
	t.Parallel()

	cfg := &MySQLConfig{
		Host:     "db.example.com",
		Port:     3306,
		User:     "alice",
		Password: "s3cr3t",
		Database: "mydb",
	}

	dsn := cfg.DSN()
	expected := "alice:s3cr3t@tcp(db.example.com:3306)/mydb?parseTime=true"
	if dsn != expected {
		t.Errorf("DSN() = %q, want %q", dsn, expected)
	}
}

func TestMySQLDSNNonStandardPort(t *testing.T) {
	t.Parallel()

	cfg := &MySQLConfig{
		Host:     "localhost",
		Port:     13306,
		User:     "root",
		Password: "",
		Database: "test",
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, ":13306)") {
		t.Errorf("DSN() should contain port 13306, got %q", dsn)
	}
}

func TestPostgreSQLDSNDefault(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv() is used

	// Ensure PGSQL_SSLMODE is not set so we get the default "prefer"
	t.Setenv("PGSQL_SSLMODE", "")

	cfg := &PostgreSQLConfig{
		Host:     "pg.example.com",
		Port:     5432,
		User:     "bob",
		Password: "hunter2",
		Database: "pgdb",
	}

	dsn := cfg.DSN()
	expected := "postgres://bob:hunter2@pg.example.com:5432/pgdb?sslmode=prefer"
	if dsn != expected {
		t.Errorf("DSN() = %q, want %q", dsn, expected)
	}
}

func TestPostgreSQLDSNCustomSSLMode(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv() is used

	t.Setenv("PGSQL_SSLMODE", "require")

	cfg := &PostgreSQLConfig{
		Host:     "supabase.example.com",
		Port:     5432,
		User:     "postgres",
		Password: "pass",
		Database: "dbname",
	}

	dsn := cfg.DSN()
	if !strings.Contains(dsn, "sslmode=require") {
		t.Errorf("DSN() should contain sslmode=require, got %q", dsn)
	}
}

func TestConfigHash(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		MySQL: MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "secret",
			Database: "mydb",
		},
		Migration: MigrationConfig{
			Source: SourceConfig{
				Table:      "users",
				PrimaryKey: "id",
				Columns:    []string{"name", "email"},
			},
			Target: TargetConfig{Table: "users"},
			Settings: SettingsConfig{
				BatchSize: 5000,
			},
		},
	}

	h1 := cfg.Hash()
	if h1 == "" {
		t.Fatal("Hash() returned empty string")
	}
	if !strings.HasPrefix(h1, "sha256:") {
		t.Errorf("Hash() should start with 'sha256:', got %q", h1)
	}

	// Same config → same hash
	h2 := cfg.Hash()
	if h1 != h2 {
		t.Errorf("Hash() is not deterministic: %q != %q", h1, h2)
	}

	// Different config → different hash
	cfg2 := *cfg
	cfg2.Migration.Source.Table = "orders"
	h3 := cfg2.Hash()
	if h1 == h3 {
		t.Error("Hash() should differ when migration config changes")
	}
}

func TestConfigHashIgnoresConnectionDetails(t *testing.T) {
	t.Parallel()

	// Two configs with same migration but different DB credentials should
	// produce the same hash (only migration block is hashed).
	cfg1 := &Config{
		MySQL:     MySQLConfig{Host: "host1", User: "user1", Password: "pw1"},
		Migration: MigrationConfig{Source: SourceConfig{Table: "tbl"}},
	}
	cfg2 := &Config{
		MySQL:     MySQLConfig{Host: "host2", User: "user2", Password: "pw2"},
		Migration: MigrationConfig{Source: SourceConfig{Table: "tbl"}},
	}

	if cfg1.Hash() != cfg2.Hash() {
		t.Error("Hash() should be the same when only connection details differ")
	}
}

// ── Env-var helpers ────────────────────────────────────────────────────────

func TestGetEnvOrDefault(t *testing.T) {
	// Cannot use t.Parallel() because subtests use t.Setenv()

	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("TEST_KEY_GOD", "myvalue")
		got := getEnvOrDefault("TEST_KEY_GOD", "default")
		if got != "myvalue" {
			t.Errorf("getEnvOrDefault() = %q, want %q", got, "myvalue")
		}
	})

	t.Run("returns default when not set", func(t *testing.T) {
		os.Unsetenv("TEST_KEY_GOD_MISSING")
		got := getEnvOrDefault("TEST_KEY_GOD_MISSING", "fallback")
		if got != "fallback" {
			t.Errorf("getEnvOrDefault() = %q, want %q", got, "fallback")
		}
	})

	t.Run("returns default when empty", func(t *testing.T) {
		t.Setenv("TEST_KEY_GOD_EMPTY", "")
		got := getEnvOrDefault("TEST_KEY_GOD_EMPTY", "fallback")
		if got != "fallback" {
			t.Errorf("getEnvOrDefault() = %q, want %q", got, "fallback")
		}
	})
}

func TestGetEnvIntOrDefault(t *testing.T) {
	// Cannot use t.Parallel() because subtests use t.Setenv()

	t.Run("returns parsed int when set", func(t *testing.T) {
		t.Setenv("TEST_PORT_GIOD", "9999")
		got := getEnvIntOrDefault("TEST_PORT_GIOD", 1234)
		if got != 9999 {
			t.Errorf("getEnvIntOrDefault() = %d, want %d", got, 9999)
		}
	})

	t.Run("returns default when not set", func(t *testing.T) {
		os.Unsetenv("TEST_PORT_GIOD_MISSING")
		got := getEnvIntOrDefault("TEST_PORT_GIOD_MISSING", 5432)
		if got != 5432 {
			t.Errorf("getEnvIntOrDefault() = %d, want %d", got, 5432)
		}
	})

	t.Run("returns default when non-integer", func(t *testing.T) {
		t.Setenv("TEST_PORT_GIOD_BAD", "not-a-number")
		got := getEnvIntOrDefault("TEST_PORT_GIOD_BAD", 3306)
		if got != 3306 {
			t.Errorf("getEnvIntOrDefault() = %d, want %d", got, 3306)
		}
	})

	t.Run("returns default when empty", func(t *testing.T) {
		t.Setenv("TEST_PORT_GIOD_EMPTY", "")
		got := getEnvIntOrDefault("TEST_PORT_GIOD_EMPTY", 8080)
		if got != 8080 {
			t.Errorf("getEnvIntOrDefault() = %d, want %d", got, 8080)
		}
	})
}

func TestNewDefaultConfig(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv() is used

	// Set well-known values
	t.Setenv("MYSQL_HOST", "mysql.test")
	t.Setenv("MYSQL_PORT", "13306")
	t.Setenv("MYSQL_USER", "testuser")
	t.Setenv("MYSQL_PASSWORD", "testpass")
	t.Setenv("MYSQL_DATABASE", "testdb")
	t.Setenv("PGSQL_HOST", "pg.test")
	t.Setenv("PGSQL_PORT", "15432")
	t.Setenv("PGSQL_USER", "pguser")
	t.Setenv("PGSQL_PASSWORD", "pgpass")
	t.Setenv("PGSQL_DATABASE", "pgdb")

	cfg := NewDefaultConfig()

	if cfg.MySQL.Host != "mysql.test" {
		t.Errorf("MySQL.Host = %q, want %q", cfg.MySQL.Host, "mysql.test")
	}
	if cfg.MySQL.Port != 13306 {
		t.Errorf("MySQL.Port = %d, want %d", cfg.MySQL.Port, 13306)
	}
	if cfg.MySQL.User != "testuser" {
		t.Errorf("MySQL.User = %q, want %q", cfg.MySQL.User, "testuser")
	}
	if cfg.MySQL.Password != "testpass" {
		t.Errorf("MySQL.Password = %q, want %q", cfg.MySQL.Password, "testpass")
	}
	if cfg.MySQL.Database != "testdb" {
		t.Errorf("MySQL.Database = %q, want %q", cfg.MySQL.Database, "testdb")
	}
	if cfg.PostgreSQL.Host != "pg.test" {
		t.Errorf("PostgreSQL.Host = %q, want %q", cfg.PostgreSQL.Host, "pg.test")
	}
	if cfg.PostgreSQL.Port != 15432 {
		t.Errorf("PostgreSQL.Port = %d, want %d", cfg.PostgreSQL.Port, 15432)
	}
	if cfg.Migration.Settings.BatchSize != 5000 {
		t.Errorf("BatchSize = %d, want %d", cfg.Migration.Settings.BatchSize, 5000)
	}
}

func TestNewDefaultConfigFallbacks(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv() is used

	// Clear all env vars — verify default values are used
	t.Setenv("MYSQL_HOST", "")
	t.Setenv("MYSQL_PORT", "")
	t.Setenv("MYSQL_USER", "")
	t.Setenv("PGSQL_HOST", "")
	t.Setenv("PGSQL_PORT", "")
	t.Setenv("PGSQL_USER", "")

	cfg := NewDefaultConfig()

	if cfg.MySQL.Host != "localhost" {
		t.Errorf("MySQL.Host default = %q, want %q", cfg.MySQL.Host, "localhost")
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("MySQL.Port default = %d, want %d", cfg.MySQL.Port, 3306)
	}
	if cfg.MySQL.User != "root" {
		t.Errorf("MySQL.User default = %q, want %q", cfg.MySQL.User, "root")
	}
	if cfg.PostgreSQL.Host != "localhost" {
		t.Errorf("PostgreSQL.Host default = %q, want %q", cfg.PostgreSQL.Host, "localhost")
	}
	if cfg.PostgreSQL.Port != 5432 {
		t.Errorf("PostgreSQL.Port default = %d, want %d", cfg.PostgreSQL.Port, 5432)
	}
	if cfg.PostgreSQL.User != "postgres" {
		t.Errorf("PostgreSQL.User default = %q, want %q", cfg.PostgreSQL.User, "postgres")
	}
}

// ── File I/O tests ─────────────────────────────────────────────────────────

func TestLoadFromPath(t *testing.T) {
	// Write a temp YAML file and load from its explicit path.
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "migration.yaml")

	yaml := `
migration:
  source:
    table: orders
    primary_key: id
    columns:
      - total
      - status
  target:
    table: orders_pg
  settings:
    batch_size: 2500
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromPath() error: %v", err)
	}

	if cfg.Migration.Source.Table != "orders" {
		t.Errorf("Source.Table = %q, want %q", cfg.Migration.Source.Table, "orders")
	}
	if cfg.Migration.Target.Table != "orders_pg" {
		t.Errorf("Target.Table = %q, want %q", cfg.Migration.Target.Table, "orders_pg")
	}
	if cfg.Migration.Settings.BatchSize != 2500 {
		t.Errorf("BatchSize = %d, want %d", cfg.Migration.Settings.BatchSize, 2500)
	}
	if len(cfg.Migration.Source.Columns) != 2 {
		t.Errorf("Columns count = %d, want %d", len(cfg.Migration.Source.Columns), 2)
	}
}

func TestLoadFromPathWithEnvExpansion(t *testing.T) {
	// Cannot use t.Parallel() because t.Setenv() is used
	t.Setenv("TEST_TABLE_NAME", "expanded_table")

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "env_expansion.yaml")

	yaml := "migration:\n  source:\n    table: $TEST_TABLE_NAME\n"
	if err := os.WriteFile(cfgFile, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPath(cfgFile)
	if err != nil {
		t.Fatalf("LoadFromPath() error: %v", err)
	}

	if cfg.Migration.Source.Table != "expanded_table" {
		t.Errorf("Source.Table = %q, want %q", cfg.Migration.Source.Table, "expanded_table")
	}
}

func TestLoadFromPathMissingFile(t *testing.T) {
	t.Parallel()

	_, err := LoadFromPath("/tmp/pgpipe-nonexistent-config-99999.yaml")
	if err == nil {
		t.Error("LoadFromPath() should return an error for a missing file")
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	// Use a temp directory as the working directory so Save() writes to
	// tmpDir/.pgpipe/config.yaml instead of the real project directory.
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cfg := &Config{
		MySQL: MySQLConfig{
			Host:     "save-test-host",
			Port:     3306,
			User:     "saveuser",
			Password: "savepass",
			Database: "savedb",
		},
		Migration: MigrationConfig{
			Source: SourceConfig{
				Table:      "save_table",
				PrimaryKey: "id",
				Columns:    []string{"a", "b"},
			},
			Target:   TargetConfig{Table: "target_table"},
			Settings: SettingsConfig{BatchSize: 999},
		},
	}

	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load back using Load() (reads from .pgpipe/config.yaml)
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.MySQL.Host != cfg.MySQL.Host {
		t.Errorf("MySQL.Host = %q, want %q", loaded.MySQL.Host, cfg.MySQL.Host)
	}
	if loaded.Migration.Source.Table != cfg.Migration.Source.Table {
		t.Errorf("Source.Table = %q, want %q", loaded.Migration.Source.Table, cfg.Migration.Source.Table)
	}
	if loaded.Migration.Settings.BatchSize != cfg.Migration.Settings.BatchSize {
		t.Errorf("BatchSize = %d, want %d", loaded.Migration.Settings.BatchSize, cfg.Migration.Settings.BatchSize)
	}
}

func TestConfigExistsAndEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	// Config should not exist yet
	if ConfigExists() {
		t.Error("ConfigExists() should be false before save")
	}

	// EnsureConfigDir should create the directory
	if err := EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir() error: %v", err)
	}

	logsDir := filepath.Join(ConfigDir, LogsDir)
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Error("EnsureConfigDir() should have created logs directory")
	}

	// Save a config to create the file
	cfg := NewDefaultConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Now config should exist
	if !ConfigExists() {
		t.Error("ConfigExists() should be true after save")
	}
}

func TestLoadReturnsDefaultWhenNoFile(t *testing.T) {
	// Use a fresh temp dir with no .pgpipe directory
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not error when config file is missing, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() should return a non-nil default config")
	}
	// Verify it returned defaults (batch size)
	if cfg.Migration.Settings.BatchSize != 5000 {
		t.Errorf("Default BatchSize = %d, want 5000", cfg.Migration.Settings.BatchSize)
	}
}
