# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.3] - 2026-06-11

### Fixed
- **Silent row loss eliminated.** `InsertBatch` previously fell back to a row-by-row insert that discarded failing rows and returned `(n, nil)` — never an error — so a migration that dropped rows reported `Skipped: 0` and exited 0. (A real 11.3M-row load silently lost 56,440 rows this way.) `InsertBatch` now surfaces the `CopyFrom` error; the migrator's per-row fallback (`retryRowsIndividually`) retries each row, logs every failure via the `ErrorLogger`, and counts it as skipped, so `processed == imported + skipped` always holds.
- **Hex-corrupted text columns.** The no-transform passthrough returned the MySQL driver's raw `[]byte`, which pgx `CopyFrom` binary-encodes as `bytea` (`\x<hex>`) into text columns — corrupting every text value (e.g. `"Teresa"` → `\x546572657361`). The passthrough now converts `[]byte`→`string` for text-family columns (same pattern as `string_to_uuid`). Note: a no-transform `BLOB`→`bytea` mapping is therefore not supported; use an explicit transform for true binary targets.
- **Error-log write failures are now fatal** instead of silently ignored: if pgpipe cannot record a skipped row, it stops rather than losing the only record of what was dropped.

### Added
- **Post-load reconciliation.** `pgpipe run` now verifies `imported + skipped == processed` with zero skips and exits non-zero (printing a `Reconcile : FAIL` line) on any gap or skip. It runs even after a fatal or cancelled run, so partial loads still surface their gap.
- Regression tests for the per-row fallback (mixed success/failure, context-cancellation) and for the reconciliation rule.

### Changed
- **TUI summary** shows a warning headline ("⚠ Migration Complete — with skipped rows (load incomplete)") instead of a green checkmark when rows were skipped.
- The per-row retry now stops promptly on context cancellation instead of attempting — and logging a failure for — every remaining row in the batch.
- `.gitignore` hardened to cover `.env.bak*` credential backups.

## [0.1.2] - 2026-02-20

### Changed
- **Module path renamed**: `github.com/pgpipe/pgpipe` → `github.com/RobertoGongora/pgpipe` to match the actual GitHub repository URL. `go install github.com/RobertoGongora/pgpipe/cmd/pgpipe@latest` now works.
- **README rewritten**: Added pre-built binary download instructions, full CLI reference (`pgpipe run` flags and per-config state behavior, `pgpipe generate-configs` all flags and example workflows), config file format with YAML examples, expanded transforms documentation, `.env` quick-start instructions.
- Updated all internal imports, `AGENTS.md`, and `DESIGN.md` to use the new module path.
- Copyright updated to 2026.

## [0.1.1] - 2026-02-20

### Added
- **`PGSQL_SSLMODE` env var**: Controls the SSL mode for PostgreSQL connections (default: `prefer`). Set to `require` for Supabase and other hosted providers that mandate SSL. Example: `PGSQL_SSLMODE=require`.
- **`.env` file support**: pgpipe now automatically loads a `.env` file from the current working directory at startup (using `godotenv.Overload`). Values in `.env` override any already-set shell environment variables, making it easy to manage connection credentials without manually sourcing the file. Works for all modes: TUI, `pgpipe run`, and `pgpipe generate-configs`.
- **Headless CLI mode**: `pgpipe run [--config=<path>]` runs a full migration without user interaction. Prints per-batch progress to stdout, exits 0 on success, non-zero on fatal error.
- **Config generator**: `pgpipe generate-configs --output-dir=<dir>` introspects MySQL + PostgreSQL schemas and writes one per-table YAML config file. Supports `--skip`, `--force`. Auto-detects transforms.
- **`string_to_uuid` transform**: Converts MySQL `CHAR(36)`/`VARCHAR(36)` UUID strings to a Go `string` that pgx can encode into PostgreSQL `uuid` columns. Validates format at the DB level (malformed UUIDs are logged per-row via ErrorLogger).
- **UUID auto-detection in TUI and generator**: `char`/`varchar` source → `uuid` target automatically sets `transform: string_to_uuid`.
- **`text_to_jsonb` auto-detection for MySQL `json` columns**: MySQL native `json`-typed columns now trigger `transform: text_to_jsonb` when the target is `jsonb` (previously only text/varchar/longtext triggered this).
- **`db.DetectTransform(srcType, tgtType)`**: Centralised transform detection function in `internal/db/types.go`, used by both TUI and CLI generator.
- **Per-config state files**: When `pgpipe run --config=./configs/foo.yaml` is used, state is saved to `./configs/.foo.state.yaml` alongside the config file, allowing concurrent runs over 85 tables without state collisions.
- Subcommand help: `pgpipe --help` prints usage for all subcommands.
- **Comprehensive test suite**: Added tests across all major packages — `internal/db` (type detection, `DetectTransform`), `internal/config` (DSN, Hash, env helpers, Load/Save round-trip), `internal/migration` (all transforms including `text_to_jsonb`, `getDefaultValueForUnmappedColumn`, `ErrorLogger` lifecycle, `StatePathForConfig`, `LoadStateFromPath`), `internal/tui` (helpers, filters, view rendering, `generateAutoMappings`), `internal/tui/styles` (`FormatNumber`).
- **GitHub Actions CI**: Runs tests, format checks, and cross-platform build verification on every push and PR.
- **GitHub Actions release workflow**: Triggered by `v*` tags; builds 4 platform binaries (linux/darwin x amd64/arm64) with checksums and creates a GitHub Release.
- **Cross-compilation in Makefile**: `make build-linux`, `make build-all` (4 platforms into `dist/`), `make coverage`.
- **Community standards**: `CONTRIBUTING.md`, `CODE_OF_CONDUCT.md` (Contributor Covenant v2.1), `SECURITY.md`, GitHub issue templates (bug report, feature request), PR template.

### Changed
- **Type detection helpers moved to `internal/db/types.go`** as exported functions (`IsTextType`, `IsIntType`, `IsBoolType`, `IsJSONType`, `IsJSONSourceType`, `IsUUIDType`). TUI wrappers in `helpers.go` now delegate to these.
- `cmd/pgpipe/main.go` now routes subcommands (`run`, `generate-configs`) before falling back to the TUI, maintaining full backward compatibility (`pgpipe` with no arguments still launches the TUI).
- `config.LoadFromPath(path)` added for loading an explicit config file path (used by CLI mode).
- `State.SetStatePath(path)` / `LoadStateFromPath(path)` / `StatePathForConfig(configPath)` added to `internal/migration/state.go` for per-config state management.
- Centralized TUI help/footer rendering with structured keybinding definitions.
- Stabilized settings screen input: `Enter` reserved for editing/selection, `s` starts migration.
- Documented refactor guidance (screen decomposition, keybinding policy, help rendering) in DESIGN.
- `.gitignore` updated to cover cross-compiled binaries (`pgpipe-*`) and `dist/` directory.
- `README.md` updated: fixed Go version prerequisite (1.22 → 1.23), added headless CLI documentation, added `PGSQL_SSLMODE` to env vars table, added `.env` file support section, added all three transforms to features list, added badges.

## [0.1.0] - 2024-01-08

### Added

#### Core Features
- Interactive TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- MySQL to PostgreSQL data migration with cursor-based pagination
- Smart column auto-matching by exact name
- Column mapping editor for fine-grained control over source→target mappings
- TEXT → JSONB transform with JSON validation
- INT → BOOL transform for MySQL tinyint(1) to PostgreSQL boolean
- Real-time migration progress display (500ms refresh rate)
- JSONL error logging per migration session
- Batch mode (run N batches then stop) and continuous mode (run until complete)
- Configuration persistence - saves progress after each wizard step
- Resume from saved configuration or start fresh

#### User Interface
- Search/filter functionality across all selection screens (press `/` to search)
  - Table search (source and target) - filter by table name
  - Column search - filter by column name AND data type
  - Case-insensitive matching
  - Real-time filtering as you type
- Scrolling viewports for long lists:
  - Tables: 10 visible rows with position indicator
  - Columns: 12 visible rows with position indicator
  - Mapping editor: 10 visible options
- Direct number input for batch settings:
  - Press Enter on batch size or batch limit to type exact value
  - Minimum value: 1 (perfect for fine-grained QA)
  - Maximum value: 2,147,483,647 (32-bit int max)
  - Input validation (0 or negative → 1, overflow → capped at max)
- Clear "Start Migration" button at bottom of settings screen
- Visual feedback for all interactive elements

#### Database Operations
- Cursor-based pagination using primary key for constant performance
- No OFFSET performance degradation on large datasets (millions of rows)
- Connection pooling for MySQL and PostgreSQL
- Schema introspection (tables, columns, types, constraints)
- Batch inserts using PostgreSQL COPY protocol (falls back to individual inserts on failure)
- Error recovery - skip invalid rows and continue migration

#### State Management
- Progressive configuration saving at each wizard step:
  1. After source table selection
  2. After column selection
  3. After target table selection
  4. After column mapping changes
  5. Before starting migration
- Migration state tracking with `.pgpipe/state.yaml`
- Config hash verification to detect configuration changes
- Resume from exact batch position using cursor value
- Per-session error logs in `.pgpipe/logs/` directory

### Fixed
- Migration progress now updates in real-time (previously stuck on "Initializing...")
- Enter key on mapping screen opens editor (previously advanced to next screen incorrectly)
- Enter key on settings screen starts migration when on "Start Migration" button
- Configuration saves progressively (previously lost on quit)
- Minimum batch values set to 1 (previously 10, preventing single-batch testing)

### Technical Details

#### Architecture
- Go 1.23 with standard library patterns
- Elm architecture for TUI (Model/Update/View)
- Ticker pattern for real-time progress updates
- No global state - all state in Model struct

#### Dependencies
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/go-sql-driver/mysql` - MySQL driver
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `gopkg.in/yaml.v3` - Configuration files

#### Performance Characteristics
- Cursor-based pagination: O(1) performance regardless of offset
- Batch processing: Configurable from 1 to 2 billion rows per batch
- Connection pooling: 10 max connections, 5 idle
- Error handling: Skip-and-continue pattern with comprehensive logging

### Documentation
- README.md with quick start guide
- DESIGN.md with technical architecture details
- AGENTS.md with guidelines for AI coding agents
- Inline code documentation for all exported types and functions

### Developer Experience
- Makefile with common commands (build, test, fmt, lint)
- Environment variable support with sensible defaults
- Example configuration file (mise.toml.example)
- Go module with pinned dependencies
