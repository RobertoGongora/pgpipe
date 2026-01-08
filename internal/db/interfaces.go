package db

import (
	"context"
	"database/sql"
)

// MySQLClientInterface defines the MySQL client interface for dependency injection
type MySQLClientInterface interface {
	Ping(ctx context.Context) error
	Close() error
	GetTables(ctx context.Context) ([]TableInfo, error)
	GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error)
	GetPrimaryKey(ctx context.Context, tableName string) (string, error)
	GetTableRowCount(ctx context.Context, tableName string) (int64, error)
	GetMinMaxID(ctx context.Context, tableName, pkColumn string) (int64, int64, error)
	FetchBatch(ctx context.Context, tableName string, columns []string, pkColumn string, cursor int64, limit int) (*sql.Rows, error)
}

// PostgresClientInterface defines the PostgreSQL client interface for dependency injection
type PostgresClientInterface interface {
	Ping(ctx context.Context) error
	Close()
	GetTables(ctx context.Context) ([]TableInfo, error)
	GetColumns(ctx context.Context, tableName string) ([]ColumnInfo, error)
	GetPrimaryKey(ctx context.Context, tableName string) (string, error)
	GetTableRowCount(ctx context.Context, tableName string) (int64, error)
	InsertBatch(ctx context.Context, tableName string, columns []string, rows [][]interface{}) (int, error)
}
