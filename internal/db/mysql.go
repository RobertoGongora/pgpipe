package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/RobertoGongora/pgpipe/internal/config"
)

// MySQLClient wraps a MySQL database connection
type MySQLClient struct {
	db     *sql.DB
	config *config.MySQLConfig
}

// TableInfo holds information about a database table
type TableInfo struct {
	Name     string
	RowCount int64
}

// ColumnInfo holds information about a table column
type ColumnInfo struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimaryKey bool
	HasDefault   bool
	DefaultValue sql.NullString
}

// NewMySQLClient creates a new MySQL client
func NewMySQLClient(cfg *config.MySQLConfig) (*MySQLClient, error) {
	db, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open MySQL connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return &MySQLClient{
		db:     db,
		config: cfg,
	}, nil
}

// Ping tests the database connection
func (c *MySQLClient) Ping(ctx context.Context) error {
	return c.db.PingContext(ctx)
}

// Close closes the database connection
func (c *MySQLClient) Close() error {
	return c.db.Close()
}

// GetTables returns a list of all tables with row counts
func (c *MySQLClient) GetTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT 
			table_name,
			table_rows
		FROM information_schema.tables 
		WHERE table_schema = ? 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := c.db.QueryContext(ctx, query, c.config.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		var rowCount sql.NullInt64
		if err := rows.Scan(&t.Name, &rowCount); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		if rowCount.Valid {
			t.RowCount = rowCount.Int64
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// GetColumns returns column information for a table
func (c *MySQLClient) GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	query := `
		SELECT 
			column_name,
			data_type,
			is_nullable,
			column_key,
			column_default
		FROM information_schema.columns 
		WHERE table_schema = ? 
		AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := c.db.QueryContext(ctx, query, c.config.Database, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var isNullable string
		var columnKey string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &columnKey, &col.DefaultValue); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		col.IsNullable = isNullable == "YES"
		col.IsPrimaryKey = columnKey == "PRI"
		col.HasDefault = col.DefaultValue.Valid
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// GetPrimaryKey returns the primary key column name for a table
func (c *MySQLClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
	columns, err := c.GetColumns(ctx, tableName)
	if err != nil {
		return "", err
	}

	for _, col := range columns {
		if col.IsPrimaryKey {
			return col.Name, nil
		}
	}

	return "", fmt.Errorf("no primary key found for table %s", tableName)
}

// GetTableRowCount returns the exact row count for a table
func (c *MySQLClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tableName)

	var count int64
	err := c.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// GetMinMaxID returns the minimum and maximum primary key values
func (c *MySQLClient) GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error) {
	query := fmt.Sprintf("SELECT COALESCE(MIN(`%s`), 0), COALESCE(MAX(`%s`), 0) FROM `%s`",
		pkColumn, pkColumn, tableName)

	var minID, maxID int64
	err := c.db.QueryRowContext(ctx, query).Scan(&minID, &maxID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get min/max ID: %w", err)
	}

	return minID, maxID, nil
}

// FetchBatch fetches a batch of rows using cursor-based pagination
func (c *MySQLClient) FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error) {
	// Build column list with PK always included first
	colList := fmt.Sprintf("`%s`", pkColumn)
	for _, col := range columns {
		if col != pkColumn {
			colList += fmt.Sprintf(", `%s`", col)
		}
	}

	query := fmt.Sprintf(
		"SELECT %s FROM `%s` WHERE `%s` > ? ORDER BY `%s` ASC LIMIT ?",
		colList, tableName, pkColumn, pkColumn,
	)

	return c.db.QueryContext(ctx, query, cursor, limit)
}

// DB returns the underlying database connection for advanced usage
func (c *MySQLClient) DB() *sql.DB {
	return c.db
}

// Config returns the MySQL configuration
func (c *MySQLClient) Config() *config.MySQLConfig {
	return c.config
}
