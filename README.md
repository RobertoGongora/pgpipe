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
- **Search & Filter** - Press `/` to search tables and columns instantly
- **Smart Column Mapping** - Auto-matches columns by name, full editor for customization
- **Cursor-Based Pagination** - Efficient pagination using primary key, no OFFSET performance degradation
- **Resumable Migrations** - State file tracks progress, resume from any point
- **Configuration Persistence** - Saves wizard selections, resume from last step or start fresh
- **Real-Time Progress** - Live updates every 500ms showing batches, rows, speed, errors
- **Batch Control** - Run 1 to 2 billion rows per batch, minimum 1 for fine-grained testing
- **Error Handling** - Skip invalid rows (e.g., bad JSON), log errors to JSONL files
- **TEXT → JSONB Transform** - Validates JSON before inserting into PostgreSQL
- **INT → BOOL Transform** - Converts MySQL tinyint(1) to PostgreSQL boolean
- **STRING → UUID Transform** - Converts MySQL CHAR/VARCHAR UUIDs to PostgreSQL uuid columns
- **Headless CLI Mode** - `pgpipe run` for scripted/automated migrations without the TUI
- **Config Generator** - `pgpipe generate-configs` introspects schemas and writes per-table configs
- **`.env` File Support** - Automatically loads `.env` from the current directory at startup

## Installation

### Prerequisites

- Go 1.24 or later
- MySQL instance (source)
- PostgreSQL instance (target)

### Build from source

```bash
git clone https://github.com/pgpipe/pgpipe.git
cd pgpipe
go mod tidy
make build
```

Or install directly:

```bash
go install github.com/pgpipe/pgpipe/cmd/pgpipe@latest
```

## Quick Start

### 1. Set environment variables

```bash
# MySQL (source)
export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_USER=root
export MYSQL_PASSWORD=secret
export MYSQL_DATABASE=source_db

# PostgreSQL (target)
export PGSQL_HOST=localhost
export PGSQL_PORT=5432
export PGSQL_USER=postgres
export PGSQL_PASSWORD=secret
export PGSQL_DATABASE=target_db
```

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

## Usage

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
| ↑/↓ or k/j | Navigate lists |
| Space | Toggle selection |
| Enter | Confirm / Continue / Edit |
| ←/→ or h/l | Adjust values |
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

pgpipe auto-matches columns by exact name:

```
Source (MySQL)              Target (PostgreSQL)
enrichment (TEXT)     →     enrichment (JSONB)     ✓ auto-matched
name (VARCHAR)        →     name (VARCHAR)         ✓ auto-matched
origin_id (VARCHAR)   →     origin_id (VARCHAR)    ✓ auto-matched
extra_field (TEXT)    →     (no match)             ✗ skipped
```

### TEXT → JSONB Transform

When migrating a TEXT column containing JSON to a JSONB column:
- pgpipe validates each value is valid JSON
- Invalid JSON rows are skipped and logged
- Migration continues without interruption

## File Structure

```
.pgpipe/                     # Created at runtime
├── config.yaml              # Saved configuration
├── state.yaml               # Migration progress
└── logs/
    └── 2024-01-08_10-30-00_errors.jsonl
```

### State File

Tracks migration progress for resumability:

```yaml
config_hash: "sha256:abc123..."
session:
  id: "2024-01-08_10-30-00"
  error_log: ".pgpipe/logs/2024-01-08_10-30-00_errors.jsonl"
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
{"mysql_id": 1234567, "error": "invalid JSON", "raw_preview": "{\"foo\": x...", "timestamp": "2024-01-08T10:30:45Z"}
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

pgpipe also supports a `.env` file in the current working directory. Values in `.env` override shell environment variables.

## Headless CLI Mode

For scripted or automated migrations, pgpipe can run without the TUI:

### Run a migration from a config file

```bash
pgpipe run --config=./configs/users.yaml
```

### Generate per-table config files from schema introspection

```bash
pgpipe generate-configs --output-dir=./configs
```

This connects to both databases, introspects all tables, and writes one YAML config per table with auto-detected column mappings and transforms.

See `pgpipe --help` for all available subcommands and flags.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Invalid JSON | Row skipped, logged to JSONL, migration continues |
| Insert failure | Row skipped, logged, migration continues |
| User quit | State saved, can resume later |
| Connection lost | Migration stops, state saved |

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

Copyright (c) 2024 Roberto Gongora
