# pgpipe - MySQL to PostgreSQL Migration Tool

A tool for migrating data from MySQL to PostgreSQL with cursor-based pagination, resumable migrations, and intelligent column mapping. Supports both an interactive TUI wizard and a headless CLI mode for scripted bulk migrations.

## Features

- **Interactive TUI** - Built with Bubble Tea for a smooth terminal experience
- **Headless CLI** - `pgpipe run --config=<path>` for scripted/batch migrations without user interaction
- **Config Generator** - `pgpipe generate-configs` introspects both schemas and writes per-table YAML config files
- **Smart Column Mapping** - Auto-matches columns by name, user can override
- **Cursor-Based Pagination** - Efficient pagination using primary key, no OFFSET performance degradation
- **Resumable Migrations** - State file tracks progress, resume from any point
- **Batch Control** - Run N batches at a time or continuous mode
- **Error Handling** - Skip invalid rows (e.g., bad JSON), log errors to JSONL files
- **Column Transforms** - TEXT → JSONB, INT → BOOL, CHAR/VARCHAR → UUID

## Installation

```bash
go install github.com/RobertoGongora/pgpipe/cmd/pgpipe@latest
```

Or build from source:

```bash
git clone https://github.com/RobertoGongora/pgpipe.git
cd pgpipe
make build
```

## Usage

### Environment Variables

Set your database credentials:

```bash
# MySQL
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=secret
export MYSQL_DATABASE=source_db

# PostgreSQL
export PGSQL_HOST=localhost
export PGSQL_PORT=5432
export PGSQL_USER=postgres
export PGSQL_PASSWORD=secret
export PGSQL_DATABASE=target_db
```

### Running pgpipe (TUI)

```bash
pgpipe
```

The TUI will guide you through:

1. **Connection Test** - Verify MySQL and PostgreSQL connections
2. **Source Selection** - Choose MySQL table and columns to migrate
3. **Target Selection** - Choose PostgreSQL target table
4. **Column Mapping** - Map source columns to target columns (auto-matched by name)
5. **Settings** - Configure batch size and run mode
6. **Migration** - Watch progress in real-time
7. **Summary** - Review results and error log location

### Resuming a Migration

If you quit mid-migration or run in batch mode, simply run `pgpipe` again. It will detect the existing state and prompt you to resume or start fresh.

### Headless CLI mode

For scripted or bulk migrations (e.g., migrating 85 tables without user interaction):

```bash
# Run a single migration from a config file
pgpipe run --config=./configs/individuals.yaml

# Uses .pgpipe/config.yaml (same as TUI) when no --config flag is given
pgpipe run
```

- Prints one progress line per batch to stdout
- Exits 0 on success, non-zero on fatal error
- State file lives alongside the config (`./configs/.individuals.state.yaml`) so
  multiple tables can run concurrently without colliding
- Resumable: re-running the same command picks up where it left off

### Generating config files

```bash
pgpipe generate-configs \
  --output-dir=./configs \
  --skip=sessions,jobs,failed_jobs,password_resets
```

- Connects to both databases and introspects all MySQL tables
- Writes one YAML file per table into `--output-dir`
- Auto-detects transforms (text_to_jsonb, int_to_bool, string_to_uuid) from column types
- Skips tables in `--skip` list, tables with no matching PG table, and existing files (unless `--force`)
- Prints a summary: N generated, M skipped (existing), K skipped (list), J skipped (no PG match)

Generated configs contain only the `migration:` block. Connection details always
come from the standard env vars (`MYSQL_*`, `PGSQL_*`) at runtime.

## Architecture

### Directory Structure

```
pgpipe/
├── cmd/pgpipe/main.go           # Entry point — subcommand routing (TUI / run / generate-configs)
├── internal/
│   ├── cli/
│   │   ├── run.go               # pgpipe run — headless migration command
│   │   └── generate.go          # pgpipe generate-configs — schema introspection + config generation
│   ├── config/
│   │   ├── config.go            # Config struct, Load/LoadFromPath/Save, Hash, DSN helpers
│   │   └── env.go               # Environment variable helpers
│   ├── db/
│   │   ├── interfaces.go        # MySQLClientInterface, PostgresClientInterface
│   │   ├── mysql.go             # MySQL connection & schema introspection
│   │   ├── postgres.go          # PostgreSQL connection & schema introspection
│   │   └── types.go             # Column type detection helpers (IsTextType, IsIntType, etc.)
│   ├── migration/
│   │   ├── migrator.go          # Core migration engine — Run, processBatch, applyTransform
│   │   ├── state.go             # State file management — Load/LoadFromPath, Save, SetStatePath
│   │   ├── errors.go            # Error logging (JSONL)
│   │   └── logger.go            # Debug log
│   └── tui/
│       ├── app.go               # Main Bubble Tea app, screen routing, generateAutoMappings
│       ├── model_state.go       # Model sub-structs
│       ├── messages.go          # tea.Msg types
│       ├── commands.go          # tea.Cmd functions
│       ├── helpers.go           # UI utilities + thin wrappers around db.IsXxx
│       ├── screen_*.go          # One file per screen
│       └── styles/              # Lipgloss styles
├── .pgpipe/                     # Default runtime directory (TUI mode)
│   ├── config.yaml              # Saved TUI configuration
│   ├── state.yaml               # Migration progress (TUI / pgpipe run default path)
│   └── logs/                    # Error logs + debug log
├── go.mod
├── Makefile
└── README.md
```

### Configuration File (.pgpipe/config.yaml)

```yaml
mysql:
  host: localhost
  port: 3306
  user: root
  password: "${MYSQL_PASSWORD}"
  database: source_db

postgres:
  host: localhost
  port: 5432
  user: postgres
  password: "${PGSQL_PASSWORD}"
  database: target_db

migration:
  source:
    table: individuals
    primary_key: id
    columns:
      - enrichment
      - name
      - origin_id

  target:
    table: individuals

  mapping:
    - source: enrichment
      target: enrichment
      transform: text_to_jsonb
    - source: is_active
      target: is_active
      transform: int_to_bool
    - source: external_id
      target: external_id
      transform: string_to_uuid
    - source: name
      target: name
    - source: origin_id
      target: origin_id

  settings:
    batch_size: 5000
```

### Per-table config (migration-only, for use with `pgpipe run --config=`)

Connection details are omitted — they always come from env vars at runtime.

```yaml
migration:
  source:
    table: individuals
    primary_key: id
    columns:
      - id
      - external_id
      - name
      - is_active
      - enrichment

  target:
    table: public.individuals

  mapping:
    - source: id
      target: id
    - source: external_id
      target: external_id
      transform: string_to_uuid
    - source: name
      target: name
    - source: is_active
      target: is_active
      transform: int_to_bool
    - source: enrichment
      target: enrichment
      transform: text_to_jsonb

  settings:
    batch_size: 5000
```

### State File (.pgpipe/state.yaml)

```yaml
config_hash: "sha256:abc123..."

session:
  id: "2024-01-08_10-30-00"
  started_at: "2024-01-08T10:30:00Z"
  error_log: ".pgpipe/logs/2024-01-08_10-30-00_errors.jsonl"

source:
  table: individuals
  total_rows: 9234567
  primary_key: id
  min_id: 1
  max_id: 9234567

progress:
  last_cursor: 4500000
  processed_rows: 4500000
  imported_rows: 4499823
  skipped_rows: 177

batches:
  size: 5000
  completed: 900

last_run:
  mode: batches
  batches_requested: 100
  batches_completed: 100
  rows_this_run: 500000
  duration_seconds: 42.3
  ended_at: "2024-01-08T10:30:42Z"
```

### Error Log Format (.pgpipe/logs/*.jsonl)

Each line is a JSON object:

```jsonl
{"mysql_id": 1234567, "error": "invalid character 'x' at position 45", "raw_preview": "{\"foo\": x...", "timestamp": "2024-01-08T10:30:45Z"}
{"mysql_id": 2345678, "error": "unexpected EOF", "raw_preview": "{\"incomplete\":", "timestamp": "2024-01-08T10:30:46Z"}
```

## Cursor-Based Pagination

Instead of using OFFSET (which degrades performance as offset increases), pgpipe uses cursor-based pagination with the primary key:

```sql
-- First batch
SELECT id, enrichment, name, origin_id 
FROM individuals 
WHERE id > 0 
ORDER BY id ASC 
LIMIT 5000;

-- Subsequent batches (cursor = last ID from previous batch)
SELECT id, enrichment, name, origin_id 
FROM individuals 
WHERE id > 5000 
ORDER BY id ASC 
LIMIT 5000;
```

Benefits:
- **Constant performance** - Each query uses the index, regardless of position in dataset
- **Deterministic** - Rows won't be missed or duplicated even if data changes
- **Resumable** - Store last cursor value, resume from exact position

## Column Mapping

### Auto-Matching

When you select a source and target table, pgpipe automatically matches columns by exact name:

```
Source (MySQL)              Target (PostgreSQL)
enrichment (TEXT)     →     enrichment (JSONB)     ✓ auto-matched
name (VARCHAR)        →     name (VARCHAR)         ✓ auto-matched
origin_id (VARCHAR)   →     origin_id (VARCHAR)    ✓ auto-matched
extra_field (TEXT)    →     (no match)             ✗ skipped
```

### Type Transforms

Currently supported:
- `TEXT → JSONB` - Validates JSON, skips invalid rows

### Unmapped Target Columns

Target columns without a mapping will:
- Use their DEFAULT value (if defined)
- Be set to NULL (if nullable)
- Cause an error (if NOT NULL without default) - pgpipe warns about this

## Run Modes

### Continuous Mode

Runs until all rows are migrated:

```
pgpipe → Select "Continuous" → Migrates all 9M rows → Shows summary → Exits
```

### Batch Mode

Runs N batches then stops (useful for controlled migrations):

```
pgpipe → Select "100 batches" → Migrates 500K rows → Shows summary → Exits
pgpipe → Resume → Another 100 batches → ...
```

## TUI Screens

1. **Welcome** - Resume existing migration or start new
2. **Connections** - Test and display MySQL/PostgreSQL connection status
3. **Source Table** - Select MySQL table (shows row counts)
4. **Source Columns** - Select columns to migrate
5. **Target Table** - Select PostgreSQL table
6. **Column Mapping** - Review/edit column mappings with type info
7. **Settings** - Batch size, run mode (continuous/N batches)
8. **Running** - Real-time progress, speed, ETA, error count
9. **Summary** - Final stats, error log location, next steps

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| ↑/↓ | Navigate lists |
| Space | Toggle selection |
| Enter | Confirm / Continue |
| q | Quit (saves progress during migration) |
| p | Pause (during migration) |
| Esc | Go back |

## Error Handling

- **Invalid JSON**: Row is skipped, logged to JSONL file, migration continues
- **Insert failure**: Row is skipped, logged, migration continues
- **Connection lost**: Migration pauses, prompts to retry or quit
- **User quit**: State is saved, can resume later

## TUI Architecture & Refactor Notes

- **Screen decomposition**: Break `internal/tui/screens.go` into focused screen modules (welcome, connections, source selection, mapping, settings, running, summary) plus reusable widgets (list/search, numeric input, mapping editor, footer/help renderer). Keep the root model as a router/compositor.
- **Async orchestration**: Move migration start-up (state load/save, PK/rowcount discovery, migrator creation) into a coordinator/service invoked via `tea.Cmd`, keeping UI update/view functions fast.
- **Keybinding policy**: Keep `Enter` for selection/advance only; use a dedicated action key for “start migration” (or a clearly labeled button) to avoid overloaded semantics. Avoid per-screen key drift; add new keys centrally.
- **Help rendering**: Use a shared helper to render key legends from structured data instead of hardcoded strings per screen. Every screen should call the helper with its bindings to keep output consistent.
- **Testing focus**: Improve mocks so batch fetching returns data; add migrator tests that exercise `Run` with transforms/skip paths; consider `teatest` for navigation and start/stop flows.

## Future Enhancements (v2)

- [ ] Automatic table creation with index replication
- [ ] Additional column transforms (dates, enums, etc.)
- [ ] Parallel batch processing across tables
- [ ] Dry-run mode
- [ ] Progress webhook notifications
- [ ] Docker image
- [ ] Shell script / Makefile generator for bulk 85-table migrations
