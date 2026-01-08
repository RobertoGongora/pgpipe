# AGENTS.md - Guidelines for AI Coding Agents

This document provides guidelines for AI agents working on the pgpipe codebase.

## Project Overview

pgpipe is a TUI-based MySQL to PostgreSQL migration tool written in Go. It uses:
- **Bubble Tea** for the terminal UI (Elm architecture: Model/Update/View)
- **pgx** for PostgreSQL connections
- **go-sql-driver/mysql** for MySQL connections
- **lipgloss** for TUI styling

## Build & Development Commands

```bash
# Build the binary
make build

# Run without building (development)
make dev
go run ./cmd/pgpipe

# Format code
make fmt
go fmt ./...

# Lint (requires golangci-lint)
make lint
golangci-lint run

# Run all tests
make test
go test -v ./...

# Run a single test
go test -v -run TestFunctionName ./internal/package/...
go test -v -run TestFunctionName ./internal/migration/...

# Run tests in a specific package
go test -v ./internal/config/...
go test -v ./internal/db/...

# Run tests with coverage
go test -cover ./...

# Download/update dependencies
make deps
go mod tidy
```

## Project Structure

```
cmd/pgpipe/main.go       # Entry point - minimal, just calls tui.Run()
internal/
  config/                # Configuration loading, env vars, YAML
  db/                    # Database clients (MySQL, PostgreSQL)
  migration/             # Core migration engine, state, error logging
  tui/                   # Bubble Tea app, screens, styles
    styles/              # Lipgloss style definitions
```

## Code Style Guidelines

### Imports

Order imports in three groups separated by blank lines:
1. Standard library
2. External dependencies
3. Internal packages

```go
import (
    "context"
    "fmt"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/jackc/pgx/v5"

    "github.com/pgpipe/pgpipe/internal/config"
    "github.com/pgpipe/pgpipe/internal/db"
)
```

### Naming Conventions

- **Packages**: lowercase, single word (e.g., `config`, `db`, `migration`)
- **Files**: lowercase with underscores (e.g., `mysql.go`, `error_logger.go`)
- **Types**: PascalCase with descriptive names (e.g., `MySQLClient`, `MigrationConfig`)
- **Interfaces**: typically end in `-er` (e.g., `ProgressCallback`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported

### Type Definitions

- Use structs with YAML tags for config/state that gets serialized
- Define custom types for enums using `type X string` with constants
- Group related types in the same file

```go
// RunMode defines how the migration should run
type RunMode string

const (
    RunModeContinuous RunMode = "continuous"
    RunModeBatches    RunMode = "batches"
)
```

### Error Handling

- Always wrap errors with context using `fmt.Errorf("description: %w", err)`
- Return `nil` for successful operations, non-nil error for failures
- Use early returns for error cases
- Log warnings for non-fatal errors, return error for fatal ones

```go
if err != nil {
    return nil, fmt.Errorf("failed to open connection: %w", err)
}
```

### Comments

- Add doc comments to all exported types, functions, and constants
- Use `// Comment` style (not `/* */`)
- Comments should explain "why", not "what"

```go
// MySQLClient wraps a MySQL database connection
type MySQLClient struct {
    db     *sql.DB
    config *config.MySQLConfig
}

// NewMySQLClient creates a new MySQL client
func NewMySQLClient(cfg *config.MySQLConfig) (*MySQLClient, error) {
```

### Bubble Tea Patterns

The TUI follows the Elm architecture:

1. **Model**: Holds all application state
2. **Update**: Handles messages and returns new state + commands
3. **View**: Renders the current state to a string

Message types should be defined as structs:
```go
type MySQLTablesMsg struct {
    tables []db.TableInfo
    err    error
}
```

Screen handlers follow the pattern:
```go
func (m Model) handleXxxKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "q":
        m.quitting = true
        return m, tea.Quit
    case "enter":
        // Handle action
    }
    return m, nil
}
```

### Database Operations

- Always use context with timeouts for database operations
- Close rows/connections with `defer`
- Use parameterized queries to prevent SQL injection
- Handle `sql.Null*` types for nullable columns

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

rows, err := c.db.QueryContext(ctx, query, args...)
if err != nil {
    return nil, fmt.Errorf("failed to query: %w", err)
}
defer rows.Close()
```

### Configuration

- Environment variables: `MYSQL_*` for MySQL, `PGSQL_*` for PostgreSQL
- Config files stored in `.pgpipe/` directory
- Use `os.ExpandEnv()` for env var substitution in YAML
- Sensitive data (passwords) should come from env vars

### State Management

- State is persisted to `.pgpipe/state.yaml`
- Save state after each batch for resumability
- Use config hash to detect configuration changes
- Error logs are JSONL files in `.pgpipe/logs/`

## Testing Guidelines

- Place tests in `*_test.go` files alongside the code
- Use table-driven tests for multiple cases
- Mock database connections for unit tests
- Use `context.Background()` in tests

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case1", "input1", "expected1"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

## Common Patterns

### Constructor Functions
```go
func NewXxx(cfg *Config) (*Xxx, error) {
    // validate, initialize, return
}
```

### Method Receivers
- Use pointer receivers `(c *Client)` for methods that modify state
- Use value receivers `(c Config)` for read-only methods on small structs

### Cleanup
- Implement `Close()` methods for resources
- Use `defer` for cleanup in the calling code
