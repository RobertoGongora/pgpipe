package db

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/pgpipe/pgpipe/internal/config"
)

// TestBuildPoolConfig verifies the pool is configured to avoid the pgx v5
// prepared-statement cache collision (SQLSTATE 42P05) that occurs when a
// connection is recycled and pgx's in-process cache diverges from the
// server-side statement list. Simple protocol sends plain-text queries and
// never registers named prepared statements, so the collision cannot happen.
func TestBuildPoolConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.PostgreSQLConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "test",
		Password: "test",
		Database: "test",
	}

	poolConfig, err := buildPoolConfig(cfg)
	if err != nil {
		t.Fatalf("buildPoolConfig returned unexpected error: %v", err)
	}

	if got := poolConfig.ConnConfig.DefaultQueryExecMode; got != pgx.QueryExecModeSimpleProtocol {
		t.Errorf("DefaultQueryExecMode = %v, want QueryExecModeSimpleProtocol", got)
	}

	if poolConfig.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", poolConfig.MaxConns)
	}

	if poolConfig.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", poolConfig.MinConns)
	}
}
