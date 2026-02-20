# AGENTS.md - Guidelines for AI Coding Agents

Guidelines for AI agents working on the pgpipe codebase.

## Project Overview

pgpipe is a MySQL to PostgreSQL migration tool written in Go. It supports both an interactive TUI wizard and a headless CLI mode. It walks the user through a wizard (source table, columns, target table, column mapping, settings) or reads a pre-written config file and runs the migration without user interaction. Both modes use cursor-based batch migration with resumability and per-row error logging.

**Key dependencies:**
- **Bubble Tea** (`bubbletea` v1.2.4) — TUI framework (Elm architecture: Model/Update/View)
- **lipgloss** (v1.0.0) — TUI styling
- **pgx** (v5.7.2) — PostgreSQL driver (uses `pgxpool` for connection pooling)
- **go-sql-driver/mysql** (v1.8.1) — MySQL driver
- **yaml.v3** — Config and state serialization

Module path: `github.com/pgpipe/pgpipe` — Go 1.24

## Build & Development Commands

```bash
make build          # go build -o ./pgpipe ./cmd/pgpipe
make dev            # go run ./cmd/pgpipe (no binary)
make test           # go test -v ./...
make fmt            # go fmt ./...
make lint           # golangci-lint run
make deps           # go mod tidy && go mod download
make clean          # rm binary + .pgpipe/ directory
make install        # cp binary to $GOPATH/bin

# Single test
go test -v -run TestFunctionName ./internal/migration/...

# Package tests
go test -v ./internal/config/...

# Coverage
go test -cover ./...
```

## Project Structure

```
cmd/pgpipe/main.go          # Entry point — subcommand routing (TUI / run / generate-configs)
internal/
  cli/
    run.go                  # pgpipe run — headless migration from config file
    generate.go             # pgpipe generate-configs — schema introspection + YAML generation
  config/
    config.go               # Config structs, Load/LoadFromPath/Save, YAML, env var expansion, Hash
    env.go                  # getEnvOrDefault helpers
  db/
    interfaces.go           # MySQLClientInterface, PostgresClientInterface
    mysql.go                # MySQLClient — FetchBatch, GetTables, GetColumns, etc.
    postgres.go             # PostgresClient — InsertBatch (pgx.CopyFrom), GetTables, etc.
    types.go                # Column type detection: IsTextType, IsIntType, IsBoolType, IsJSONType,
                            #   IsJSONSourceType, IsUUIDType, DetectTransform
    mocks_test.go           # Package-internal mocks (not importable outside db)
    mysql_test.go           # MySQL client tests
  migration/
    migrator.go             # Core engine — Run, processBatch, transformRow, applyTransform
    state.go                # State persistence — Save/Load/LoadFromPath, SetStatePath, StatePathForConfig
    errors.go               # ErrorLogger — per-row JSONL error logging
    logger.go               # Debug log (debug.log) — lazy-opened, mutex-protected
    migrator_test.go        # Migrator + transform tests
    state_test.go           # State management tests
  testutil/
    mocks.go                # Mock MySQL/Postgres clients (importable by all packages)
    fixtures.go             # Test data generators (tables, columns, batch data, config)
    helpers.go              # AssertNoError, AssertEqual, AssertTrue, etc.
  tui/
    app.go                  # Model struct, Init/Update/View, screen routing, Run()
    model_state.go          # Sub-structs: ConnectionState, DataCache, SelectionState, etc.
    messages.go             # All tea.Msg types (ConnectionTestMsg, MigrationProgressMsg, etc.)
    commands.go             # All tea.Cmd functions (connectDatabases, startMigration, etc.)
    constants.go            # UI constants (viewport sizes, batch limits, tick intervals)
    helpers.go              # UI utilities + thin wrappers delegating to db.IsXxx helpers
    help.go                 # Shared help footer renderer
    screen_welcome.go       # Resume/new choice screen
    screen_connections.go   # Database connection status
    screen_source_table.go  # MySQL table picker with search
    screen_source_columns.go # Column multi-select with search
    screen_target_table.go  # PostgreSQL table picker with search
    screen_mapping.go       # Column mapping editor with auto-detect transforms
    screen_settings.go      # Batch size, run mode, batch limit configuration
    screen_running.go       # Live migration progress display
    screen_summary.go       # Final results
    styles/styles.go        # All lipgloss style definitions and colors
```

## Architecture

### Subcommand Routing

`cmd/pgpipe/main.go` checks `os.Args[1]` and routes:

| Invocation | Behaviour |
|-----------|-----------|
| `pgpipe` (no args) | Launch interactive TUI |
| `pgpipe run [--config=path]` | Headless migration via `cli.RunMigration()` |
| `pgpipe generate-configs [flags]` | Schema introspection + config generation via `cli.GenerateConfigs()` |
| `pgpipe --help` | Print usage |
| `pgpipe <unknown>` | Fall back to TUI (backward compat) |

### Migration Flow

```
MySQL                             PostgreSQL
  │                                  │
  │  FetchBatch (cursor-based)       │
  │  WHERE pk > ? ORDER BY pk LIMIT ?│
  │◄─────────────────────────────────│
  │                                  │
  │  transformRow() per row          │
  │    └─ applyTransform() per col   │
  │                                  │
  │  InsertBatch (pgx.CopyFrom)     │
  │─────────────────────────────────►│
  │                                  │
  │  State saved after each batch    │
  │  (.pgpipe/state.yaml or          │
  │   configs/.foo.state.yaml)       │
```

**`Migrator.Run()`** in `migrator.go` is the main loop:
1. Initializes from `state.Progress.LastCursor` (enables resume).
2. Loops: checks stop signal / context / batch limit / completion.
3. Each iteration calls `processBatch()` which fetches, transforms, inserts.
4. After each batch: updates state, saves to disk, fires progress callback.
5. On exit: calls `finalize()` (end run, save state, close logger).

**`processBatch()`** in `migrator.go`:
1. `mysql.FetchBatch()` — cursor-based pagination.
2. Scans each row, extracts PK, calls `transformRow()`.
3. If transform fails: logs error via `ErrorLogger`, skips row.
4. Bulk inserts via `postgres.InsertBatch()` (uses `pgx.CopyFrom`).
5. If bulk insert fails: falls back to individual row inserts, logging per-row failures.

### Transform Pipeline

Transforms convert MySQL values to PostgreSQL-compatible Go types.

**`transformRow()`** in `migrator.go` maps source columns to target columns using `config.ColumnMapping`, calling `applyTransform()` for each mapped column. Unmapped NOT NULL target columns get smart defaults via `getDefaultValueForUnmappedColumn()`.

**`applyTransform()`** in `migrator.go` switches on the transform name:

| Transform | MySQL source | PG target | Go coercion |
|-----------|-------------|-----------|-------------|
| `text_to_jsonb` | text/varchar/longtext/json | json/jsonb | Validates JSON via `json.Valid()`, passes string through |
| `int_to_bool` | tinyint/smallint/int/bigint | boolean | `int64 != 0` → bool; passes bool through; nil → nil |
| `string_to_uuid` | char/varchar (UUID format) | uuid | Converts `[]byte`/`string` to Go `string`; pgx sends as text; PG validates format |
| `""` / `"none"` | any | any | Pass-through |

Transform names are case-insensitive (both lowercase and UPPERCASE accepted). Unknown transform names return an error at runtime — there is no config-level validation.

**Note on UUID coercion**: pgx does not auto-coerce `[]byte` or `string` to `uuid` via `CopyFrom`. The `string_to_uuid` transform converts MySQL `[]byte` to a Go `string`, which pgx encodes via the text protocol. PostgreSQL then validates the UUID format and rejects malformed values per-row (caught by the ErrorLogger).

### Type Detection Helpers

All column type detection lives in `internal/db/types.go` as exported functions:

| Function | Side | Matches |
|----------|------|---------|
| `IsTextType(t)` | MySQL source | `text`, `mediumtext`, `longtext`, `varchar`, `char` |
| `IsJSONSourceType(t)` | MySQL source | `json` (native MySQL JSON column) |
| `IsJSONType(t)` | PG target | `json`, `jsonb` |
| `IsIntType(t)` | MySQL source | `tinyint`, `smallint`, `mediumint`, `int`, `bigint` |
| `IsBoolType(t)` | PG target | `boolean`, `bool` |
| `IsUUIDType(t)` | PG target | `uuid` |
| `DetectTransform(src, tgt)` | both | Returns transform name or `""` |

TUI's `tui/helpers.go` exports thin lowercase wrappers (`isTextType` etc.) that delegate to these, so TUI code does not need to import `db` for type checks alone.

### Auto-Detection in TUI and Generator

When columns are matched (by name) in `generateAutoMappings()` in `app.go`, manually in the mapping editor in `screen_mapping.go`, or during `generate-configs`, transforms are auto-detected via `db.DetectTransform()`:

```
(IsTextType || IsJSONSourceType) && IsJSONType  → "text_to_jsonb"
IsIntType && IsBoolType                          → "int_to_bool"
IsTextType && IsUUIDType                         → "string_to_uuid"
```

### Per-Config State Files (CLI mode)

When `pgpipe run --config=./configs/foo.yaml` is used, the state file lives alongside the config as `./configs/.foo.state.yaml`. This allows 85+ tables to run (sequentially or concurrently) without colliding on `.pgpipe/state.yaml`.

When no `--config` is specified, the default `.pgpipe/state.yaml` is used (TUI-compatible behaviour).

Key functions in `migration/state.go`:
- `SetStatePath(path)` — override save path at runtime
- `LoadStateFromPath(path)` — load state from an explicit path
- `StatePathForConfig(configPath)` — derive state path from config path

### TUI Screen Flow

```
Welcome ─► Connections ─► SourceTable ─► SourceColumns ─► TargetTable ─► Mapping ─► Settings ─► Running ─► Summary
   │                                                                                    ▲
   └─── Resume / Use saved config ──────────────────────────────────────────────────────┘
```

Screens are an enum (`Screen` type in `app.go`). Each screen has:
- A `viewXxx()` method returning a string (renders UI)
- A `handleXxxKeys(msg tea.KeyMsg)` method (handles input, returns model + cmd)

The `Model.Update()` method routes messages to the current screen's handler. Screen transitions are done by setting `m.screen = ScreenXxx`.

### Model Structure

The top-level `Model` in `app.go` uses semantic sub-structs defined in `model_state.go`:

| Sub-struct | Purpose |
|-----------|---------|
| `conn ConnectionState` | MySQL/PG connection status and errors |
| `data DataCache` | Cached table/column metadata from both databases |
| `selection SelectionState` | User's table, column, and mapping choices |
| `settings SettingsState` | Batch size, run mode, batch limit, editing state |
| `migration MigrationState` | Active migrator instance, stats, done flag |
| `ui UIState` | Ephemeral UI state: resume choice, editor state, search mode |

### State & Resumability

State is persisted to the state file path after every batch. On restart, the migrator picks up from `state.Progress.LastCursor`. A config hash (`config.Hash()`) detects if the migration config changed since the last run.

### Configuration

- **TUI config**: `.pgpipe/config.yaml` — saved after each wizard step.
- **CLI per-table config**: any YAML file with only a `migration:` block; connection details come from env vars.
- **Env vars**: `MYSQL_HOST`, `MYSQL_PORT`, `MYSQL_USER`, `MYSQL_PASSWORD`, `MYSQL_DATABASE`, `PGSQL_HOST`, `PGSQL_PORT`, `PGSQL_USER`, `PGSQL_PASSWORD`, `PGSQL_DATABASE`.
- **Env expansion**: `os.ExpandEnv()` is called on YAML content at load time, so `$MYSQL_PASSWORD` works in the config file.
- **Sensitive data**: Passwords should come from env vars, not be hardcoded in YAML.
- `config.Load()` — reads `.pgpipe/config.yaml` (TUI default).
- `config.LoadFromPath(path)` — reads an explicit path (CLI mode); connection fields fall back to env vars for any fields not present in the file.

### Error Logging

Per-session JSONL files in `.pgpipe/logs/{sessionID}_errors.jsonl`. Each entry has: `MySQLID`, `Error`, `RawPreview` (truncated to 100 chars), `Timestamp`. The `ErrorLogger` is thread-safe (mutex-protected), tracks count and last 10 errors in memory.

### Database Interfaces

Both `MySQLClientInterface` and `PostgresClientInterface` in `db/interfaces.go` are interfaces, enabling mock implementations for testing. Key asymmetry: MySQL has `FetchBatch()` (source), PostgreSQL has `InsertBatch()` (target).

## Code Style Guidelines

### Imports

Three groups separated by blank lines: stdlib, external, internal.

```go
import (
    "context"
    "fmt"

    tea "github.com/charmbracelet/bubbletea"

    "github.com/pgpipe/pgpipe/internal/config"
)
```

### Naming

- **Packages**: lowercase, single word (`config`, `db`, `migration`, `cli`)
- **Files**: lowercase with underscores (`error_logger.go`, `screen_mapping.go`)
- **Types**: PascalCase (`MySQLClient`, `MigrationConfig`)
- **Interfaces**: end in `-er` or `-Interface` (`ProgressCallback`, `MySQLClientInterface`)
- **Enums**: `type X string` with `const` block (`RunMode`, `Screen`)

### Error Handling

Always wrap with context: `fmt.Errorf("failed to X: %w", err)`. Use early returns. Log warnings for non-fatal errors (skipped rows), return errors for fatal ones.

### Bubble Tea Patterns

Messages are structs with data + optional error:
```go
type MySQLTablesMsg struct {
    tables []db.TableInfo
    err    error
}
```

Commands are functions returning `tea.Cmd` (closures that perform I/O and return a message). Screen handlers return `(tea.Model, tea.Cmd)`.

### Database Operations

Always use context with timeouts. Close rows/connections with `defer`. Use parameterized queries.

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

## Testing Guidelines

### Test Infrastructure

- **`internal/testutil/`** — Importable mocks and helpers used by all packages.
- **`NewMockMySQLClient()`** — Pre-configured with 5 tables, 4 columns, 10000 rows.
- **`NewMockPostgresClient()`** — Pre-configured with 5 tables, InsertedCount=1000.
- Both mocks track call counts (e.g., `mock.Calls.FetchBatch`) for verification.
- **`testutil.AssertNoError(t, err)`**, `AssertEqual`, `AssertTrue`, etc.
- **`testutil.GenerateMockBatchData(rows, cols, startID)`** — Creates test row data.
- **`testutil.CreateTestConfig()`** — Returns a full config for localhost databases.

### Test Patterns

Table-driven tests with `t.Parallel()`:

```go
func TestXxx(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name      string
        input     interface{}
        expected  interface{}
        expectErr bool
    }{
        {"case1", input1, expected1, false},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}
```

To test unexported methods (like `applyTransform`), tests must be in the same package (e.g., `package migration` in `migrator_test.go`).

### What Has Test Coverage

| Package | Covered | Notable gaps |
|---------|---------|-------------|
| `internal/db` | Mock clients, MySQL client | No integration tests (need real DBs) |
| `internal/migration` | Migrator creation/stop, state CRUD, all three transforms, transformRow | `Run()` / `processBatch()` not exercised (mock FetchBatch returns nil `*sql.Rows`) |
| `internal/tui` | Migration start flow, message handling | No screen rendering or navigation tests |
| `internal/cli` | No tests yet | RunMigration and GenerateConfigs require live DB connections |

## Common Modification Recipes

### Adding a New Transform

1. **`internal/db/types.go`** — Add `IsXxxSourceType()` and/or `IsYyyTargetType()` helpers for source/target type detection. Add a new `else if` branch in `DetectTransform()`.

2. **`internal/migration/migrator.go`** — Add a `case "name", "NAME":` to `applyTransform()`, before the `"", "none"` passthrough. Handle all expected Go types from MySQL + nil + default error.

3. **`internal/migration/migrator_test.go`** — Add `TestApplyTransformXxx` with table-driven subtests covering: expected conversions, passthrough types, nil → nil, unexpected type → error.

4. **`internal/tui/helpers.go`** — Add thin wrapper functions (e.g. `isXxxType`) delegating to `db.IsXxxType`. No logic here.

5. **`internal/tui/screen_mapping.go`** — Add a warning display in the warnings loop in `viewMapping()`.

6. **Docs** — Update `DESIGN.md` (config example + roadmap), `CHANGELOG.md`, `AGENTS.md` transforms table.

7. **Run `go test ./...`** to verify.

Note: `tui/app.go` and `tui/screen_mapping.go` auto-detection now derive from `db.DetectTransform()` indirectly through `isXxxType` wrappers, but the `else if` chain in both places needs manual extension. Keep them in sync with `db.DetectTransform()`.

### Adding a New Screen

1. Create `internal/tui/screen_xxx.go` with `viewXxx()` and `handleXxxKeys()` methods on `Model`.
2. Add `ScreenXxx` to the `Screen` enum in `app.go`.
3. Add routing in `Model.Update()` and `Model.View()`.
4. Add any new sub-struct fields to `model_state.go`.
5. Add any new messages to `messages.go` and commands to `commands.go`.

### Adding a New Database Method

1. Add the method to the appropriate interface in `db/interfaces.go`.
2. Implement it in `db/mysql.go` or `db/postgres.go`.
3. Add it to the mock in `internal/testutil/mocks.go` (importable mock).
4. Add it to the mock in `db/mocks_test.go` (package-internal mock) if db tests need it.

### Adding a New CLI Subcommand

1. Create `internal/cli/xxx.go` with a `func Xxx(args []string) error` entry point.
2. Use `flag.NewFlagSet("xxx", flag.ContinueOnError)` for flag parsing.
3. Add a `case "xxx":` in `cmd/pgpipe/main.go`'s switch.
4. Update `usage` const in `main.go`.
5. Update `AGENTS.md` subcommand routing table, `DESIGN.md`, `CHANGELOG.md`.

## Key Constants

| Constant | Value | Location |
|----------|-------|----------|
| `MigrationTickInterval` | 500ms | `tui/constants.go` |
| `ConnectionTimeout` | 5s | `tui/constants.go` |
| `QueryTimeout` | 30s | `tui/constants.go` |
| `MaxVisibleTables` | 10 | `tui/constants.go` |
| `MaxVisibleColumns` | 12 | `tui/constants.go` |
| `MaxVisibleTargets` | 10 | `tui/constants.go` |
| `DefaultBatchSize` | 5000 | `config/config.go` |
| MySQL pool | MaxOpen=10, MaxIdle=5 | `db/mysql.go` |
| PG pool | MaxConns=10, MinConns=2 | `db/postgres.go` |
