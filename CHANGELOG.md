# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2024-01-08

### Added

#### Core Features
- Interactive TUI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea)
- MySQL to PostgreSQL data migration with cursor-based pagination
- Smart column auto-matching by exact name
- Column mapping editor for fine-grained control over source→target mappings
- TEXT → JSONB transform with JSON validation
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
