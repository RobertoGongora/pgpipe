# pgpipe Refactoring Proposal

This document outlines findings from a comprehensive code review of the pgpipe codebase, comparing it against Go best practices and Bubble Tea architectural patterns. It includes specific recommendations for refactoring to improve maintainability, testability, and code organization.

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Current Architecture Analysis](#current-architecture-analysis)
3. [Issues Identified](#issues-identified)
4. [Refactoring Recommendations](#refactoring-recommendations)
5. [Testing Improvements](#testing-improvements)
6. [Implementation Plan](#implementation-plan)

---

## Executive Summary

The pgpipe codebase is generally well-structured and follows many Go conventions. However, there are several areas that would benefit from refactoring:

| Priority | Issue | Impact |
|----------|-------|--------|
| **High** | `screens.go` is 1848 lines | Maintenance burden, cognitive load |
| **High** | Duplicate mock implementations | Test maintenance overhead |
| **Medium** | Large monolithic Model struct | Tight coupling, hard to test |
| **Medium** | Magic numbers in UI code | Readability |
| **Low** | Missing table-driven tests for handlers | Test coverage gaps |

---

## Current Architecture Analysis

### File Size Distribution

```
internal/tui/screens.go        1848 lines  [CRITICAL]
internal/tui/app.go             568 lines
internal/migration/migrator.go  459 lines
internal/migration/migrator_test.go 299 lines
internal/db/postgres.go         241 lines
internal/migration/state_test.go 225 lines
internal/db/mocks_test.go       218 lines
internal/db/mysql_test.go       216 lines
internal/db/mysql.go            204 lines
internal/config/config.go       184 lines
internal/migration/state.go     182 lines
```

### Bubble Tea Architecture Review

The current implementation follows the Elm architecture (Model-Update-View) correctly:

**What's Working Well:**
- Clean separation of message types in `app.go`
- Proper use of `tea.Cmd` for async operations
- Interface-based database clients enable testing
- Good use of `tea.Batch` for concurrent operations

**Areas for Improvement:**
- Screen logic is not separated (all in one 1848-line file)
- Model struct has ~30 fields mixing concerns
- Commands are defined inline with screens rather than centralized

### Go Best Practices Compliance

| Practice | Status | Notes |
|----------|--------|-------|
| Import ordering | Pass | Standard lib, external, internal |
| Error wrapping | Pass | Uses `fmt.Errorf("...: %w", err)` |
| Interface design | Pass | Small interfaces (e.g., `db.MySQLClientInterface`) |
| Package naming | Pass | Lowercase, single-word names |
| Constructor functions | Pass | `NewX()` pattern used |
| Zero value usefulness | Partial | Some structs require explicit init |

---

## Issues Identified

### 1. `screens.go` is Too Large (1848 lines)

**Problem:** The file contains all view functions and key handlers for 9 screens plus a modal editor. This violates the single responsibility principle and makes the code hard to navigate.

**Contents of `screens.go`:**
- `viewWelcome()` + `handleWelcomeKeys()` (~100 lines)
- `viewConnections()` + `handleConnectionsKeys()` + `connectDatabases()` (~180 lines)
- `viewSourceTable()` + `handleSourceTableKeys()` (~200 lines)
- `viewSourceColumns()` + `handleSourceColumnsKeys()` (~250 lines)
- `viewTargetTable()` + `handleTargetTableKeys()` (~200 lines)
- `viewMapping()` + `handleMappingKeys()` (~100 lines)
- `viewMappingEditor()` + `handleMappingEditorKeys()` (~150 lines)
- `viewSettings()` + `handleSettingsKeys()` (~300 lines)
- `viewRunning()` + `handleRunningKeys()` (~200 lines)
- `viewSummary()` + `handleSummaryKeys()` (~100 lines)
- Helper functions (~100 lines)

**Impact:**
- Difficult to find specific screen logic
- Higher risk of merge conflicts
- Cognitive overload when working on any screen

### 2. Model Struct Has Mixed Concerns (~30 fields)

**Problem:** The `Model` struct in `app.go` mixes:
- Navigation state (`screen`, `quitting`)
- Database connections (`mysqlClient`, `pgClient`)
- Connection status (`mysqlConnected`, `pgConnected`, `mysqlError`, `pgError`)
- Table/column data (`mysqlTables`, `pgTables`, `mysqlColumns`, `pgColumns`)
- User selections (`sourceTable`, `targetTable`, `selectedColumns`, `columnMappings`)
- UI state (`tableCursor`, `columnCursor`, `mappingCursor`, `editingBatchSize`)
- Search state (`searchMode`, `searchQuery`, `filteredColumns`)
- Migration runtime (`migrator`, `migrationStats`, `migrationDone`)

**Impact:**
- Hard to understand what state belongs to which screen
- Testing requires setting up many unrelated fields
- Tight coupling between screens

### 3. Duplicate Mock Implementations

**Problem:** Mock clients are defined in three places:

1. **`db/mocks_test.go`** (218 lines) - Full-featured mocks with call tracking:
   ```go
   type MockMySQLClient struct {
       PingError    error
       Tables       []TableInfo
       // ... many fields
       PingCalled   int
       GetTablesCalled int
       // ... call tracking
   }
   ```

2. **`migration/migrator_test.go`** (48 lines) - Minimal inline mocks:
   ```go
   type mockMySQLClient struct {
       tables   []db.TableInfo
       columns  []db.ColumnInfo
       rowCount int64
       minID    int64
       maxID    int64
   }
   ```

3. **`tui/testing_test.go`** (75 lines) - Another minimal mock set:
   ```go
   type mockMySQLClient struct {
       rowCount int64
       minID    int64
       maxID    int64
       columns  []db.ColumnInfo
   }
   ```

**Impact:**
- Changes to interfaces require updates in 3 places
- Inconsistent mock behavior across tests
- Wasted code

### 4. Magic Numbers in UI Code

**Examples found:**

```go
// screens.go:377
visibleCount := 10  // Not a named constant

// screens.go:585
const maxVisibleColumns = 12  // Good - but inconsistent

// screens.go:1124
const maxVisibleTargets = 10  // Good

// screens.go:828
visibleCount := 10  // Same magic number, different location
```

**Impact:**
- Inconsistent viewport sizes
- Hard to adjust UI parameters globally

### 5. Inconsistent Command Organization

**Problem:** Async commands (`tea.Cmd` functions) are scattered:
- `connectDatabases()` in `screens.go:157`
- `loadMySQLTables()` in `screens.go:232`
- `loadPGTables()` in `screens.go:242`
- `loadMySQLColumns()` in `screens.go:522`
- `loadPGColumns()` in `screens.go:981`
- `startMigration()` in `screens.go` (at bottom)
- `tickAfter()` in `app.go:555`
- `waitForMigrationCompletion()` in `app.go:562`

**Best Practice (from Charm blog):**
> "Use commands for all I/O... Commands are a fundamental part of Bubble Tea."

Commands should be organized together for discoverability.

---

## Refactoring Recommendations

### Recommendation 1: Split screens.go into Per-Screen Files

**Proposed Structure:**
```
internal/tui/
├── app.go                 # Model, Init, Update dispatch, View dispatch
├── commands.go            # All tea.Cmd functions
├── messages.go            # All message type definitions  
├── constants.go           # UI constants (viewport sizes, etc.)
├── helpers.go             # truncate(), progressBar(), filter functions
├── screens/
│   ├── welcome.go         # viewWelcome, handleWelcomeKeys
│   ├── connections.go     # viewConnections, handleConnectionsKeys
│   ├── source_table.go    # viewSourceTable, handleSourceTableKeys
│   ├── source_columns.go  # viewSourceColumns, handleSourceColumnsKeys
│   ├── target_table.go    # viewTargetTable, handleTargetTableKeys
│   ├── mapping.go         # viewMapping, handleMappingKeys
│   ├── mapping_editor.go  # viewMappingEditor, handleMappingEditorKeys
│   ├── settings.go        # viewSettings, handleSettingsKeys
│   ├── running.go         # viewRunning, handleRunningKeys
│   └── summary.go         # viewSummary, handleSummaryKeys
└── styles/
    └── styles.go          # (unchanged)
```

**Implementation Notes:**
- Each screen file exports `View*` and `Handle*Keys` functions
- Functions take `*Model` as receiver (stays in main package)
- This is the recommended Bubble Tea pattern for larger apps

**Alternative (Simpler):**
Keep everything in `internal/tui/` but split into separate files:
```
internal/tui/
├── app.go
├── commands.go
├── constants.go
├── helpers.go
├── screen_welcome.go
├── screen_connections.go
├── screen_source_table.go
├── screen_source_columns.go
├── screen_target_table.go
├── screen_mapping.go
├── screen_settings.go
├── screen_running.go
├── screen_summary.go
└── styles/styles.go
```

This is simpler and avoids sub-package complexity.

### Recommendation 2: Group Model Fields into Sub-Structs

**Current:**
```go
type Model struct {
    screen      Screen
    config      *config.Config
    mysqlClient db.MySQLClientInterface
    pgClient    db.PostgresClientInterface
    mysqlConnected bool
    pgConnected    bool
    mysqlError     string
    pgError        string
    mysqlTables  []db.TableInfo
    // ... 25+ more fields
}
```

**Proposed:**
```go
type Model struct {
    // Core
    screen   Screen
    config   *config.Config
    quitting bool
    width    int
    height   int
    err      error

    // Grouped state
    connections ConnectionState
    data        DataState
    selection   SelectionState
    ui          UIState
    migration   MigrationState
}

type ConnectionState struct {
    MySQL  ClientConnection
    PG     ClientConnection
}

type ClientConnection struct {
    Client    interface{} // MySQLClientInterface or PostgresClientInterface
    Connected bool
    Error     string
}

type DataState struct {
    MySQLTables  []db.TableInfo
    PGTables     []db.TableInfo
    MySQLColumns []db.ColumnInfo
    PGColumns    []db.ColumnInfo
}

type SelectionState struct {
    SourceTable     string
    SourceTableIdx  int
    TargetTable     string
    TargetTableIdx  int
    SelectedColumns map[string]bool
    ColumnMappings  []config.ColumnMapping
}

type UIState struct {
    // Cursors
    TableCursor   int
    ColumnCursor  int
    MappingCursor int
    SettingsCursor int
    
    // Editing modes
    EditingBatchSize  bool
    EditingBatchLimit bool
    EditingMapping    bool
    InputBuffer       string
    
    // Search
    SearchMode       bool
    SearchQuery      string
    FilteredColumns  []db.ColumnInfo
    TableSearchMode  bool
    TableSearchQuery string
    FilteredTables   []db.TableInfo
}

type MigrationState struct {
    Migrator     *migration.Migrator
    State        *migration.State
    Stats        migration.MigrationStats
    Done         bool
    HasExisting  bool
    ResumeChoice int
    
    // Settings
    BatchSize  int
    RunMode    migration.RunMode
    BatchLimit int
}
```

**Benefits:**
- Clear ownership of fields
- Easier to pass subsets to functions
- Better documentation through structure
- Simpler initialization

### Recommendation 3: Consolidate Mock Implementations

**Proposed:** Create a shared `testutil/mocks.go` file:

```go
// internal/testutil/mocks.go
package testutil

import (
    "context"
    "database/sql"
    "github.com/pgpipe/pgpipe/internal/db"
)

// MockMySQLClient provides a configurable mock for testing
type MockMySQLClient struct {
    // Responses
    PingErr      error
    Tables       []db.TableInfo
    TablesErr    error
    Columns      []db.ColumnInfo
    ColumnsErr   error
    RowCount     int64
    RowCountErr  error
    MinID, MaxID int64
    MinMaxErr    error
    BatchRows    *sql.Rows
    BatchErr     error

    // Call tracking (optional, for tests that need it)
    Calls MockCalls
}

type MockCalls struct {
    Ping         int
    Close        int
    GetTables    int
    GetColumns   int
    GetRowCount  int
    GetMinMax    int
    FetchBatch   int
}

func (m *MockMySQLClient) Ping(ctx context.Context) error {
    m.Calls.Ping++
    return m.PingErr
}

// ... implement all methods

// NewMockMySQLClient creates a mock with sensible defaults
func NewMockMySQLClient() *MockMySQLClient {
    return &MockMySQLClient{
        Tables:   GenerateMockTableInfo(5),
        Columns:  GenerateMockColumnInfo(4, true),
        RowCount: 10000,
        MinID:    1,
        MaxID:    10000,
    }
}
```

**Benefits:**
- Single source of truth for mocks
- Consistent behavior across all tests
- Changes to interfaces require one update
- Call tracking available when needed

### Recommendation 4: Extract UI Constants

**Create `internal/tui/constants.go`:**

```go
package tui

// Viewport sizes
const (
    MaxVisibleTables  = 10
    MaxVisibleColumns = 12
    MaxVisibleTargets = 10
)

// Input validation
const (
    MinBatchSize = 1
    MaxBatchSize = 2147483647  // int32 max
    MinBatchLimit = 1
    MaxBatchLimit = 2147483647
)

// Refresh rates
const (
    MigrationTickInterval = 500 * time.Millisecond
)
```

### Recommendation 5: Centralize Commands

**Create `internal/tui/commands.go`:**

```go
package tui

import (
    "context"
    "time"
    tea "github.com/charmbracelet/bubbletea"
)

// Database commands
func (m Model) cmdConnectDatabases() tea.Msg { ... }
func (m Model) cmdLoadMySQLTables() tea.Msg { ... }
func (m Model) cmdLoadPGTables() tea.Msg { ... }
func (m Model) cmdLoadMySQLColumns() tea.Msg { ... }
func (m Model) cmdLoadPGColumns() tea.Msg { ... }

// Migration commands  
func (m Model) cmdStartMigration() tea.Cmd { ... }
func cmdTickAfter(d time.Duration) tea.Cmd { ... }
func cmdWaitForCompletion(done chan error) tea.Cmd { ... }
```

---

## Testing Improvements

### Issue 1: No Table-Driven Tests for Key Handlers

**Current:** Tests focus on message flow, not key handling branches.

**Recommendation:** Add comprehensive table-driven tests:

```go
func TestSourceTableKeys(t *testing.T) {
    tests := []struct {
        name           string
        initialState   func() Model
        key            string
        expectedScreen Screen
        expectedCursor int
        expectQuit     bool
    }{
        {
            name: "q quits",
            initialState: func() Model {
                m := createTestModel()
                m.screen = ScreenSourceTable
                return m
            },
            key:        "q",
            expectQuit: true,
        },
        {
            name: "down moves cursor",
            initialState: func() Model {
                m := createTestModel()
                m.screen = ScreenSourceTable
                m.mysqlTables = []db.TableInfo{{Name: "a"}, {Name: "b"}}
                m.tableCursor = 0
                return m
            },
            key:            "down",
            expectedCursor: 1,
        },
        {
            name: "enter selects table",
            initialState: func() Model {
                m := createTestModel()
                m.screen = ScreenSourceTable
                m.mysqlTables = []db.TableInfo{{Name: "users"}}
                return m
            },
            key:            "enter",
            expectedScreen: ScreenSourceColumns,
        },
        // ... more cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            m := tt.initialState()
            newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
            // Assert expectations
        })
    }
}
```

### Issue 2: Mock Setup is Complex

**Current:** Tests in `tui/testing_test.go` define minimal mocks that duplicate `db/mocks_test.go`.

**Problem:** The `FetchBatch` method returns `*sql.Rows` which is hard to mock.

**Recommendation:** Consider refactoring `FetchBatch` to return a more testable type:

```go
// Current (hard to test)
FetchBatch(ctx context.Context, ...) (*sql.Rows, error)

// Option A: Return slice (breaks abstraction but simplifies tests)
FetchBatch(ctx context.Context, ...) ([][]interface{}, error)

// Option B: Add iterator interface
type RowIterator interface {
    Next() bool
    Scan(dest ...interface{}) error
    Close() error
    Err() error
}
FetchBatch(ctx context.Context, ...) (RowIterator, error)
```

Option B maintains the streaming behavior while allowing easy mocking.

### Issue 3: Test Helpers Not in testutil

**Current:** `createTestModel()` and `executeTestCmd()` are in `tui/testing_test.go`.

**Recommendation:** Move TUI-specific test helpers to `testutil/tui.go`:

```go
// internal/testutil/tui.go
package testutil

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/pgpipe/pgpipe/internal/tui"
)

// ExecuteCmd runs a Bubble Tea command synchronously
func ExecuteCmd(cmd tea.Cmd) tea.Msg {
    if cmd == nil {
        return nil
    }
    return cmd()
}

// CreateTestTUIModel creates a pre-configured Model for testing
func CreateTestTUIModel() tui.Model {
    // ... implementation
}
```

---

## Implementation Plan

### Phase 1: Low-Risk Improvements (1-2 hours)

1. **Extract constants** to `internal/tui/constants.go`
   - Risk: Very low
   - Impact: Improved readability

2. **Move messages** to `internal/tui/messages.go`
   - Risk: Low (just moving code)
   - Impact: Better organization

### Phase 2: Mock Consolidation (2-3 hours)

1. **Create `testutil/mocks.go`** with unified mock implementations
2. **Update tests** to use shared mocks
3. **Remove duplicate mocks** from `migration/` and `tui/` test files

### Phase 3: Screen Splitting (3-4 hours)

1. **Create `internal/tui/commands.go`** - centralize all tea.Cmd functions
2. **Create per-screen files** (simpler approach without sub-package):
   - `screen_welcome.go`
   - `screen_connections.go`
   - `screen_source_table.go`
   - `screen_source_columns.go`
   - `screen_target_table.go`
   - `screen_mapping.go`
   - `screen_settings.go`
   - `screen_running.go`
   - `screen_summary.go`
3. **Delete `screens.go`**

### Phase 4: Model Restructuring (2-3 hours) [Optional]

1. **Define sub-structs** (`ConnectionState`, `UIState`, etc.)
2. **Update Model** to use sub-structs
3. **Update all references** throughout the codebase
4. **Update tests**

This phase is optional as it's more invasive. Consider if Phase 3 sufficiently improves maintainability.

### Phase 5: Test Improvements (2-3 hours)

1. **Add table-driven handler tests**
2. **Move TUI test helpers to testutil**
3. **Consider `RowIterator` interface** for better `FetchBatch` testing

---

## Summary

The pgpipe codebase is fundamentally sound but would benefit from:

1. **Splitting the large `screens.go` file** - This is the highest-impact change
2. **Consolidating mock implementations** - Reduces maintenance burden
3. **Extracting constants** - Quick win for readability
4. **Adding table-driven handler tests** - Improves test coverage

The recommendations follow Go best practices as outlined in:
- [Effective Go](https://go.dev/doc/effective_go)
- [Practical Go by Dave Cheney](https://dave.cheney.net/practical-go)
- [Bubble Tea documentation](https://github.com/charmbracelet/bubbletea)

Estimated total effort: **10-15 hours** for full implementation, or **3-5 hours** for Phase 1-2 only.
