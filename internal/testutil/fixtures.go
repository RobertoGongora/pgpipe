package testutil

import (
	"database/sql"
	"fmt"

	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
)

// GenerateMockTableInfo creates test table data
func GenerateMockTableInfo(count int) []db.TableInfo {
	tables := make([]db.TableInfo, count)
	for i := 0; i < count; i++ {
		tables[i] = db.TableInfo{
			Name:     fmt.Sprintf("table_%d", i+1),
			RowCount: int64((i + 1) * 1000),
		}
	}
	return tables
}

// GenerateMockColumnInfo creates test column data
func GenerateMockColumnInfo(numCols int, withPK bool) []db.ColumnInfo {
	columns := make([]db.ColumnInfo, numCols)
	for i := 0; i < numCols; i++ {
		isPK := withPK && i == 0
		columns[i] = db.ColumnInfo{
			Name:         fmt.Sprintf("col_%d", i+1),
			DataType:     "varchar",
			IsNullable:   !isPK,
			IsPrimaryKey: isPK,
			HasDefault:   false,
			DefaultValue: sql.NullString{},
		}
	}
	return columns
}

// GenerateMockBatchData creates test row data
func GenerateMockBatchData(numRows, numCols int, startID int64) [][]interface{} {
	rows := make([][]interface{}, numRows)
	for i := 0; i < numRows; i++ {
		row := make([]interface{}, numCols)
		row[0] = startID + int64(i) // First column is always PK
		for j := 1; j < numCols; j++ {
			row[j] = fmt.Sprintf("value_%d_%d", i, j)
		}
		rows[i] = row
	}
	return rows
}

// CreateTestConfig creates a test configuration
func CreateTestConfig() *config.Config {
	return &config.Config{
		MySQL: config.MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			Database: "test_db",
			User:     "test_user",
			Password: "test_pass",
		},
		PostgreSQL: config.PostgreSQLConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "test_db",
			User:     "test_user",
			Password: "test_pass",
		},
		Migration: config.MigrationConfig{
			Source: config.SourceConfig{
				Table:      "users",
				PrimaryKey: "id",
				Columns:    []string{"name", "email", "created_at"},
			},
			Target: config.TargetConfig{
				Table: "users",
			},
			Mapping: []config.ColumnMapping{
				{Source: "name", Target: "name"},
				{Source: "email", Target: "email"},
				{Source: "created_at", Target: "created_at"},
			},
			Settings: config.SettingsConfig{
				BatchSize: 1000,
			},
		},
	}
}

// Note: CreateTestState, CreateTestMigrationConfig, and SetupTestClients are defined in their
// respective package test files to avoid import cycles
