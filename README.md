# pgpipe

[![CI](https://github.com/RobertoGongora/pgpipe/actions/workflows/ci.yml/badge.svg)](https://github.com/RobertoGongora/pgpipe/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/RobertoGongora/pgpipe)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A MySQL to PostgreSQL migration tool with an interactive TUI wizard and a headless CLI mode. Cursor-based pagination, resumable migrations, and intelligent column mapping.

```
┌──────────────────────────────────────────────────────────────┐
│  pgpipe - Migration in Progress                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  individuals (MySQL) → individuals (PostgreSQL)              │
│                                                              │
│  Progress: [████████████████░░░░░░░░] 67.2%                  │
│                                                              │
│  Batches:     67 / 100                                       │
│  Rows:        335,000 / 500,000 (this run)                   │
│  Total:       4,835,000 / 9,234,567 (overall)                │
│                                                              │
│  Speed:       12,450 rows/sec                                │
│  Skipped:     177 (invalid JSON)                             │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

## Features

- **Interactive TUI** - Built with Bubble Tea for a smooth terminal experience
- **Headless CLI Mode** - `pgpipe run` for scripted/automated migrations without the TUI
- **Config Generator** - `pgpipe generate-configs` introspects schemas and writes per-table configs
- **Search & Filter** - Press `/` to search tables and columns instantly
- **Smart Column Mapping** - Auto-matches columns by name, full editor for customization
- **Cursor-Based Pagination** - Efficient pagination using primary key, no OFFSET performance degradation
- **Resumable Migrations** - State file tracks progress, resume from any point
- **Configuration Persistence** - Saves wizard selections, resume from last step or start fresh
- **Real-Time Progress** - Live updates every 500ms showing batches, rows, speed, errors
- **Batch Control** - Run 1 to 2 billion rows per batch, minimum 1 for fine-grained testing
- **Error Handling** - Skip invalid rows (e.g., bad JSON), log errors to JSONL files
- **`.env` File Support** - Automatically loads `.env` from the current directory at startup

### Column Transforms

pgpipe auto-detects and applies transforms when column types differ between MySQL and PostgreSQL:

| Transform | MySQL Source | PostgreSQL Target | Behavior |
|-----------|-------------|-------------------|----------|
| `text_to_jsonb` | `TEXT`, `VARCHAR`, `LONGTEXT`, `JSON` | `JSON`, `JSONB` | Validates JSON; invalid rows are skipped and logged |
| `int_to_bool` | `TINYINT`, `SMALLINT`, `INT`, `BIGINT` | `BOOLEAN` | `0` → `false`, non-zero → `true`, `NULL` → `NULL` |
| `string_to_uuid` | `CHAR`, `VARCHAR` | `UUID` | Passes UUID string through; PostgreSQL validates format |

Transforms are auto-detected in both the TUI wizard and `generate-configs`. You can also set them manually in config files.

## Installation

### Download a pre-built binary

Pre-built binaries are available for Linux and macOS on the [Releases](https://github.com/RobertoGongora/pgpipe/releases) page.

```bash
# Linux (amd64)
curl -L https://github.com/RobertoGongora/pgpipe/releases/latest/download/pgpipe-linux-amd64 -o pgpipe
chmod +x pgpipe

# macOS (Apple Silicon)
curl -L https://github.com/RobertoGongora/pgpipe/releases/latest/download/pgpipe-darwin-arm64 -o pgpipe
chmod +x pgpipe

# macOS (Intel)
curl -L https://github.com/RobertoGongora/pgpipe/releases/latest/download/pgpipe-darwin-amd64 -o pgpipe
chmod +x pgpipe
```

### Install with `go install`

```bash
go install github.com/RobertoGongora/pgpipe/cmd/pgpipe@latest
```

Requires Go 1.24 or later.

### Build from source

```bash
git clone https://github.com/RobertoGongora/pgpipe.git
cd pgpipe
make build
```

## Quick Start

### 1. Set environment variables

Create a `.env` file in your working directory (or export the variables directly):

```bash
# MySQL (source)
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=secret
MYSQL_DATABASE=source_db

# PostgreSQL (target)
PGSQL_HOST=localhost
PGSQL_PORT=5432
PGSQL_USER=postgres
PGSQL_PASSWORD=secret
PGSQL_DATABASE=target_db
```

pgpipe automatically loads `.env` from the current directory at startup.

### 2. Run pgpipe

```bash
./pgpipe
```

### 3. Follow the TUI wizard

1. **Connections** - Verify MySQL and PostgreSQL connections
2. **Source Table** - Select MySQL table (shows row counts)
3. **Source Columns** - Select columns to migrate
4. **Target Table** - Select PostgreSQL target table
5. **Column Mapping** - Review auto-matched mappings, edit if needed
6. **Settings** - Choose batch size and run mode
7. **Migration** - Watch real-time progress
8. **Summary** - Review results and error log location

## CLI Reference

pgpipe has three modes of operation:

```
pgpipe                              Launch the interactive TUI wizard
pgpipe run [--config=<path>]        Run a migration headlessly from a config file
pgpipe generate-configs [flags]     Generate per-table config files from live schemas
pgpipe --help                       Print usage information
```

### `pgpipe` (TUI mode)

Launches the interactive terminal wizard. This is the default when no subcommand is given. Configuration is saved to `.pgpipe/config.yaml` and migration state to `.pgpipe/state.yaml`.

### `pgpipe run`

Runs a migration headlessly from a config file. Useful for scripted, automated, or parallel migrations.

```
Usage: pgpipe run [--config=<path>]

Flags:
  --config string   Path to the migration config YAML file (default: .pgpipe/config.yaml)
```

**Default behavior** (no `--config` flag):
```bash
pgpipe run
```
Reads `.pgpipe/config.yaml` and saves state to `.pgpipe/state.yaml` — the same paths used by the TUI.

**Per-table config** (with `--config` flag):
```bash
pgpipe run --config=./configs/users.yaml
```
Reads the specified config file. The state file is automatically placed alongside the config as `./configs/.users.state.yaml`. This allows running many tables in parallel without state file collisions.

The CLI always runs in continuous mode (processes all rows to completion). If a previous run was interrupted, it resumes from the last saved cursor position. If the config has changed since the last run (detected via SHA-256 hash), it starts fresh.

**Example output:**
```
[pgpipe] Starting migration: users → users (batch_size=5000)
[pgpipe] Connected to MySQL and PostgreSQL
[pgpipe] Source: 1,234,567 rows (id 1..1234567)
[pgpipe] users: batch 1 | 5,000/1,234,567 (0.4%) | imported=5,000 skipped=0 | elapsed=2s
[pgpipe] users: batch 2 | 10,000/1,234,567 (0.8%) | imported=10,000 skipped=0 | elapsed=4s
...
[pgpipe] Migration complete: users
  Processed : 1,234,567 rows
  Imported  : 1,234,390 rows
  Skipped   : 177 rows
  Duration  : 8m32.451s
  Errors    : 177 (see .pgpipe/logs/2026-02-20_10-30-00_errors.jsonl)
```

### `pgpipe generate-configs`

Connects to both databases, introspects all tables, and writes one YAML config per matching table with auto-detected column mappings and transforms.

```
Usage: pgpipe generate-configs --output-dir=<dir> [flags]

Flags:
  --output-dir string   Directory to write generated config files (required)
  --skip string         Comma-separated list of table names to skip
  --force               Overwrite existing config files (default: skip existing)
```

**Example:**
```bash
# Generate configs for all tables
pgpipe generate-configs --output-dir=./configs

# Skip certain tables
pgpipe generate-configs --output-dir=./configs --skip=migrations,sessions,cache

# Regenerate all configs (overwrite existing)
pgpipe generate-configs --output-dir=./configs --force
```

Tables are matched by name between MySQL and PostgreSQL. Tables with no matching PostgreSQL table are skipped. The generated configs contain only the `migration:` block — connection details always come from environment variables.

**Example workflow — migrate all tables:**
```bash
# 1. Generate configs
pgpipe generate-configs --output-dir=./configs

# 2. Review and adjust (edit transforms, remove tables you don't want)
ls ./configs/

# 3. Run them sequentially
for f in ./configs/*.yaml; do
  pgpipe run --config="$f"
done

# 4. Or run them in parallel (each has its own state file)
for f in ./configs/*.yaml; do
  pgpipe run --config="$f" &
done
wait
```

### Config File Format

Config files used by `pgpipe run` follow this YAML structure. Files generated by `generate-configs` contain only the `migration:` block (connection details come from env vars):

```yaml
migration:
  source:
    table: users
    primary_key: id
    columns:
      - id
      - name
      - email
      - metadata
      - is_active
      - external_id
  target:
    table: public.users
  mapping:
    - source: id
      target: id
    - source: name
      target: name
    - source: email
      target: email
    - source: metadata
      target: metadata
      transform: text_to_jsonb
    - source: is_active
      target: is_active
      transform: int_to_bool
    - source: external_id
      target: external_id
      transform: string_to_uuid
  settings:
    batch_size: 5000
```

Connection details can also be included (passwords should use env var references):

```yaml
mysql:
  host: localhost
  port: 3306
  user: root
  password: $MYSQL_PASSWORD
  database: source_db
postgres:
  host: localhost
  port: 5432
  user: postgres
  password: $PGSQL_PASSWORD
  database: target_db
```

Environment variables in YAML values (like `$MYSQL_PASSWORD`) are expanded at load time.

## TUI Usage

### Run Modes

**Continuous Mode**: Runs until all rows are migrated
```
pgpipe → Select "Continuous" → Migrates all rows → Exits
```

**Batch Mode**: Runs N batches then stops (useful for controlled migrations)
```
pgpipe → Select "100 batches" → Migrates ~500K rows → Exits
pgpipe → Resume → Another 100 batches → ...
```

### Resuming a Migration

pgpipe saves your configuration and progress automatically:

```bash
./pgpipe
# First run after setup: "Saved configuration found!"
# Shows: source → target, column count, batch size
# Options: "Use saved configuration" or "Start fresh"

# Mid-migration: "Existing migration found!"
# Shows: progress %, rows processed, last run time
# Options: "Resume migration" or "Start new migration"
```

Configuration is saved after each major step, so you can:
- Quit during wizard setup - resume from where you left off
- Run partial migrations - continue from last batch
- Modify saved config - navigate with ESC to change any setting

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| / | Search/filter (tables, columns) |
| Up/Down or k/j | Navigate lists |
| Space | Toggle selection |
| Enter | Confirm / Continue / Edit |
| Left/Right or h/l | Adjust values |
| a | Select all (columns) |
| n | Select none (columns) |
| c | Continue (mapping screen) |
| e | Edit number (settings screen) |
| Esc | Go back / Exit search |
| q | Quit (saves progress during migration) |
| Ctrl+C | Force quit |

## How It Works

### Cursor-Based Pagination

Instead of using OFFSET (which degrades performance as offset increases), pgpipe uses cursor-based pagination:

```sql
-- First batch
SELECT id, enrichment FROM individuals WHERE id > 0 ORDER BY id LIMIT 5000;

-- Next batch (cursor = last ID from previous batch)
SELECT id, enrichment FROM individuals WHERE id > 5000 ORDER BY id LIMIT 5000;
```

Benefits:
- **Constant performance** - Each query uses the index
- **Deterministic** - No missed or duplicate rows
- **Resumable** - Store cursor value, resume from exact position

### Column Mapping

pgpipe auto-matches columns by exact name and detects transforms based on type pairs:

```
Source (MySQL)              Target (PostgreSQL)         Transform
enrichment (TEXT)     →     enrichment (JSONB)          text_to_jsonb (auto)
is_active (TINYINT)  →     is_active (BOOLEAN)         int_to_bool (auto)
ext_id (CHAR(36))    →     ext_id (UUID)               string_to_uuid (auto)
name (VARCHAR)        →     name (VARCHAR)              (none)
extra_field (TEXT)    →     (no match)                  skipped
```

## File Structure

```
.pgpipe/                     # Created at runtime
├── config.yaml              # Saved configuration
├── state.yaml               # Migration progress
└── logs/
    └── 2026-02-20_10-30-00_errors.jsonl
```

### State File

Tracks migration progress for resumability:

```yaml
config_hash: "sha256:abc123..."
session:
  id: "2026-02-20_10-30-00"
  error_log: ".pgpipe/logs/2026-02-20_10-30-00_errors.jsonl"
source:
  table: individuals
  total_rows: 9234567
  primary_key: id
progress:
  last_cursor: 4500000
  processed_rows: 4500000
  imported_rows: 4499823
  skipped_rows: 177
```

### Error Log (JSONL)

Each line is a JSON object for easy parsing:

```jsonl
{"mysql_id": 1234567, "error": "invalid JSON", "raw_preview": "{\"foo\": x...", "timestamp": "2026-02-20T10:30:45Z"}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MYSQL_HOST` | localhost | MySQL host |
| `MYSQL_PORT` | 3306 | MySQL port |
| `MYSQL_USER` | root | MySQL user |
| `MYSQL_PASSWORD` | - | MySQL password |
| `MYSQL_DATABASE` | - | MySQL database |
| `PGSQL_HOST` | localhost | PostgreSQL host |
| `PGSQL_PORT` | 5432 | PostgreSQL port |
| `PGSQL_USER` | postgres | PostgreSQL user |
| `PGSQL_PASSWORD` | - | PostgreSQL password |
| `PGSQL_DATABASE` | - | PostgreSQL database |
| `PGSQL_SSLMODE` | prefer | PostgreSQL SSL mode (`prefer`, `require`, `disable`) |

Set `PGSQL_SSLMODE=require` for hosted providers like Supabase that mandate SSL connections.

pgpipe loads `.env` from the current working directory at startup. Values in `.env` override existing shell environment variables.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid JSON value | Row skipped, logged to JSONL, migration continues |
| Malformed UUID | Row skipped, logged, migration continues |
| Insert failure | Row skipped, logged, migration continues |
| User quit (q) | State saved, can resume later |
| Connection lost | Migration stops, state saved at last successful batch |

## Development

```bash
make build        # Build for current platform
make build-linux  # Cross-compile for linux/amd64
make build-all    # Build all 4 platforms (linux/darwin x amd64/arm64) into dist/
make dev          # Run from source (go run)
make test         # Run all tests
make coverage     # Show test coverage per package
make fmt          # Format code
make lint         # Run golangci-lint
make clean        # Remove binaries and runtime data
```

## Roadmap

Future enhancements planned for v2:

- [ ] Automatic table creation with index replication
- [ ] Additional column transforms (dates, enums, etc.)
- [ ] Parallel batch processing
- [ ] Dry-run mode with migration preview
- [ ] Progress webhook notifications
- [ ] Docker image
- [ ] Migration templates (save/load complete configurations)
- [ ] Rollback support

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, coding standards, and the pull request process.

For bug reports and feature requests, please [open an issue](https://github.com/RobertoGongora/pgpipe/issues) using the provided templates.

## License

MIT License - see [LICENSE](LICENSE) file for details.

Copyright (c) 2026 Roberto Gongora
