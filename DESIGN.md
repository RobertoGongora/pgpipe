# pgpipe - MySQL to PostgreSQL Migration Tool

A TUI-based tool for migrating data from MySQL to PostgreSQL with cursor-based pagination, resumable migrations, and intelligent column mapping.

## Features

- **Interactive TUI** - Built with Bubble Tea for a smooth terminal experience
- **Smart Column Mapping** - Auto-matches columns by name, user can override
- **Cursor-Based Pagination** - Efficient pagination using primary key, no OFFSET performance degradation
- **Resumable Migrations** - State file tracks progress, resume from any point
- **Batch Control** - Run N batches at a time or continuous mode
- **Error Handling** - Skip invalid rows (e.g., bad JSON), log errors to JSONL files
- **TEXT → JSONB Transform** - Validates JSON before inserting into PostgreSQL

## Installation

```bash
go install github.com/pgpipe/pgpipe/cmd/pgpipe@latest
```

Or build from source:

```bash
git clone https://github.com/pgpipe/pgpipe.git
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

### Running pgpipe

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

## Architecture

### Directory Structure

```
pgpipe/
├── cmd/pgpipe/main.go           # Entry point
├── internal/
│   ├── config/
│   │   ├── config.go            # Config struct & loading
│   │   └── env.go               # Environment variable expansion
│   ├── db/
│   │   ├── mysql.go             # MySQL connection & schema introspection
│   │   └── postgres.go          # PostgreSQL connection & schema introspection
│   ├── migration/
│   │   ├── migrator.go          # Core migration engine
│   │   ├── state.go             # State file management
│   │   └── errors.go            # Error logging (JSONL)
│   └── tui/
│       ├── app.go               # Main Bubble Tea app
│       ├── model.go             # App state model
│       ├── screens/             # Individual screens
│       ├── components/          # Reusable UI components
│       └── styles/              # Lipgloss styles
├── .pgpipe/                     # Runtime directory (created at runtime)
│   ├── config.yaml              # Saved configuration
│   ├── state.yaml               # Migration progress
│   └── logs/                    # Error logs
│       └── 2024-01-08_10-30-00_errors.jsonl
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
    - source: name
      target: name
    - source: origin_id
      target: origin_id

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
- [ ] Parallel batch processing
- [ ] Dry-run mode
- [ ] Progress webhook notifications
- [ ] Docker image
