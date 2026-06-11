package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresClient wraps a PostgreSQL database connection pool
type PostgresClient struct {
	pool   *pgxpool.Pool
	config *config.PostgreSQLConfig
}

// buildPoolConfig constructs a pgxpool.Config from the given PostgreSQL config.
// Extracted so that pool settings (including QueryExecMode) can be verified
// in unit tests without requiring a live database connection.
func buildPoolConfig(cfg *config.PostgreSQLConfig) (*pgxpool.Config, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to parse PostgreSQL DSN: %w", err)
	}

	// Configure connection pool
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2

	// Use simple query protocol to avoid pgx v5's prepared statement cache
	// colliding with server-side state after connection resets. CopyFrom
	// (used by InsertBatch) uses the COPY protocol and is unaffected.
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	return poolConfig, nil
}

// NewPostgresClient creates a new PostgreSQL client
func NewPostgresClient(cfg *config.PostgreSQLConfig) (*PostgresClient, error) {
	poolConfig, err := buildPoolConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL connection pool: %w", err)
	}

	return &PostgresClient{
		pool:   pool,
		config: cfg,
	}, nil
}

// Ping tests the database connection
func (c *PostgresClient) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// Close closes the database connection pool
func (c *PostgresClient) Close() {
	c.pool.Close()
}

// GetTables returns a list of all tables with row counts
func (c *PostgresClient) GetTables(ctx context.Context) ([]TableInfo, error) {
	query := `
		SELECT 
			schemaname || '.' || relname as table_name,
			n_live_tup as row_count
		FROM pg_stat_user_tables
		ORDER BY schemaname, relname
	`

	rows, err := c.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.RowCount); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// GetColumns returns column information for a table
func (c *PostgresClient) GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	// Parse schema.table format
	schema := "public"
	table := tableName
	if parts := strings.SplitN(tableName, ".", 2); len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	query := `
		SELECT 
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES' as is_nullable,
			COALESCE(
				(SELECT true FROM information_schema.table_constraints tc
				 JOIN information_schema.key_column_usage kcu 
				 ON tc.constraint_name = kcu.constraint_name
				 WHERE tc.table_schema = c.table_schema 
				 AND tc.table_name = c.table_name
				 AND kcu.column_name = c.column_name
				 AND tc.constraint_type = 'PRIMARY KEY'
				 LIMIT 1), 
			false) as is_primary_key,
			c.column_default IS NOT NULL as has_default,
			c.column_default
		FROM information_schema.columns c
		WHERE c.table_schema = $1 
		AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := c.pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to query columns: %w", err)
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var defaultVal *string
		if err := rows.Scan(&col.Name, &col.DataType, &col.IsNullable, &col.IsPrimaryKey, &col.HasDefault, &defaultVal); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		if defaultVal != nil {
			col.DefaultValue.String = *defaultVal
			col.DefaultValue.Valid = true
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// GetPrimaryKey returns the primary key column name for a table
func (c *PostgresClient) GetPrimaryKey(ctx context.Context, tableName string) (string, error) {
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
func (c *PostgresClient) GetTableRowCount(ctx context.Context, tableName string) (int64, error) {
	// Parse schema.table format
	schema := "public"
	table := tableName
	if parts := strings.SplitN(tableName, ".", 2); len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s"."%s"`, schema, table)

	var count int64
	err := c.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// InsertBatch inserts multiple rows into a table
// Returns the number of rows successfully inserted and any error
func (c *PostgresClient) InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}

	// Parse schema.table format
	schema := "public"
	table := tableName
	if parts := strings.SplitN(tableName, ".", 2); len(parts) == 2 {
		schema = parts[0]
		table = parts[1]
	}

	// Bulk COPY (much faster than individual INSERTs). On failure, return the
	// error instead of silently retrying row-by-row and discarding the rows that
	// fail — the caller decides how to recover. Never swallow a COPY error here:
	// doing so was the silent row-loss bug this replaced.
	copyCount, err := c.pool.CopyFrom(
		ctx,
		pgx.Identifier{schema, table},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("COPY into %q.%q (%d rows) failed: %w", schema, table, len(rows), err)
	}

	return int(copyCount), nil
}

// Pool returns the underlying connection pool for advanced usage
func (c *PostgresClient) Pool() *pgxpool.Pool {
	return c.pool
}

// Config returns the PostgreSQL configuration
func (c *PostgresClient) Config() *config.PostgreSQLConfig {
	return c.config
}
