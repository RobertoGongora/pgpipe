package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir  = ".pgpipe"
	ConfigFile = "config.yaml"
	StateFile  = "state.yaml"
	LogsDir    = "logs"
)

// Config represents the main configuration for pgpipe
type Config struct {
	MySQL      MySQLConfig      `yaml:"mysql"`
	PostgreSQL PostgreSQLConfig `yaml:"postgres"`
	Migration  MigrationConfig  `yaml:"migration"`
}

// MySQLConfig holds MySQL connection settings
type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// PostgreSQLConfig holds PostgreSQL connection settings
type PostgreSQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// MigrationConfig holds migration-specific settings
type MigrationConfig struct {
	Source   SourceConfig    `yaml:"source"`
	Target   TargetConfig    `yaml:"target"`
	Mapping  []ColumnMapping `yaml:"mapping"`
	Settings SettingsConfig  `yaml:"settings"`
}

// SourceConfig defines the source table and columns
type SourceConfig struct {
	Table      string   `yaml:"table"`
	PrimaryKey string   `yaml:"primary_key"`
	Columns    []string `yaml:"columns"`
}

// TargetConfig defines the target table
type TargetConfig struct {
	Table string `yaml:"table"`
}

// ColumnMapping defines how a source column maps to a target column
type ColumnMapping struct {
	Source    string `yaml:"source"`
	Target    string `yaml:"target"`
	Transform string `yaml:"transform,omitempty"` // e.g., "text_to_jsonb"
}

// SettingsConfig holds runtime settings
type SettingsConfig struct {
	BatchSize int `yaml:"batch_size"`
}

// NewDefaultConfig creates a config with default values from environment
func NewDefaultConfig() *Config {
	return &Config{
		MySQL: MySQLConfig{
			Host:     getEnvOrDefault("MYSQL_HOST", "localhost"),
			Port:     getEnvIntOrDefault("MYSQL_PORT", 3306),
			User:     getEnvOrDefault("MYSQL_USER", "root"),
			Password: os.Getenv("MYSQL_PASSWORD"),
			Database: os.Getenv("MYSQL_DATABASE"),
		},
		PostgreSQL: PostgreSQLConfig{
			Host:     getEnvOrDefault("PGSQL_HOST", "localhost"),
			Port:     getEnvIntOrDefault("PGSQL_PORT", 5432),
			User:     getEnvOrDefault("PGSQL_USER", "postgres"),
			Password: os.Getenv("PGSQL_PASSWORD"),
			Database: os.Getenv("PGSQL_DATABASE"),
		},
		Migration: MigrationConfig{
			Settings: SettingsConfig{
				BatchSize: 5000,
			},
		},
	}
}

// Load reads config from the config file, falling back to defaults
func Load() (*Config, error) {
	cfg := NewDefaultConfig()

	configPath := filepath.Join(ConfigDir, ConfigFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults from env
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML
	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// LoadFromPath reads a config file from an explicit path, falling back to env
// var defaults for any fields not present in the file. This allows per-table
// migration configs that omit the mysql/postgres connection blocks — those are
// always filled in from the environment.
func LoadFromPath(path string) (*Config, error) {
	cfg := NewDefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Expand environment variables in the YAML
	expanded := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// Save writes the config to the config file
func (c *Config) Save() error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(ConfigDir, ConfigFile)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Hash returns a SHA256 hash of the migration-relevant config
// Used to detect if config has changed between runs
func (c *Config) Hash() string {
	// Only hash the migration config, not connection details
	data, _ := yaml.Marshal(c.Migration)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}

// EnsureConfigDir creates the config directory if it doesn't exist
func EnsureConfigDir() error {
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	logsPath := filepath.Join(ConfigDir, LogsDir)
	if err := os.MkdirAll(logsPath, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	return nil
}

// ConfigExists checks if a config file exists
func ConfigExists() bool {
	configPath := filepath.Join(ConfigDir, ConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}

// MySQLDSN returns the MySQL connection string
func (c *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		c.User, c.Password, c.Host, c.Port, c.Database)
}

// PostgresDSN returns the PostgreSQL connection string.
// SSL mode is controlled by the PGSQL_SSLMODE environment variable
// (default: "prefer"). Set PGSQL_SSLMODE=require for Supabase and other
// hosted providers that mandate SSL.
func (c *PostgreSQLConfig) DSN() string {
	sslmode := getEnvOrDefault("PGSQL_SSLMODE", "prefer")
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Database, sslmode)
}
