package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/db"
)

// RunMode defines how the migration should run
type RunMode string

const (
	RunModeContinuous RunMode = "continuous"
	RunModeBatches    RunMode = "batches"
)

// MigrationConfig holds the configuration for a migration run
type MigrationConfig struct {
	SourceTable   string
	TargetTable   string
	SourcePK      string
	SourceColumns []string
	TargetColumns []string
	Mapping       []config.ColumnMapping
	BatchSize     int
	Mode          RunMode
	BatchLimit    int // Only used when Mode == RunModeBatches
}

// MigrationStats holds statistics about the migration progress
type MigrationStats struct {
	BatchesCompleted int
	RowsProcessed    int64
	RowsImported     int64
	RowsSkipped      int64
	CurrentCursor    int64
	StartTime        time.Time
	LastBatchTime    time.Time
	TotalRows        int64
	ErrorLogPath     string
}

// ProgressCallback is called after each batch with current stats
type ProgressCallback func(stats MigrationStats)

// Migrator handles the migration process
type Migrator struct {
	mysql         db.MySQLClientInterface
	postgres      db.PostgresClientInterface
	config        MigrationConfig
	state         *State
	errorLogger   *ErrorLogger
	onProgress    ProgressCallback
	stopChan      chan struct{}
	stopped       bool
	targetColumns map[string]db.ColumnInfo // Cache of target column metadata
}

// NewMigrator creates a new migrator instance
func NewMigrator(
	mysql db.MySQLClientInterface,
	postgres db.PostgresClientInterface,
	cfg MigrationConfig,
	state *State,
) (*Migrator, error) {
	errorLogger, err := NewErrorLogger(state.Session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create error logger: %w", err)
	}

	// Load target column metadata for smart default handling
	ctx := context.Background()
	targetCols, err := postgres.GetColumns(ctx, cfg.TargetTable)
	if err != nil {
		return nil, fmt.Errorf("failed to get target columns: %w", err)
	}

	// Build a map for quick lookup
	targetColMap := make(map[string]db.ColumnInfo)
	for _, col := range targetCols {
		targetColMap[col.Name] = col
	}

	return &Migrator{
		mysql:         mysql,
		postgres:      postgres,
		config:        cfg,
		state:         state,
		errorLogger:   errorLogger,
		stopChan:      make(chan struct{}),
		targetColumns: targetColMap,
	}, nil
}

// SetProgressCallback sets the callback for progress updates
func (m *Migrator) SetProgressCallback(cb ProgressCallback) {
	m.onProgress = cb
}

// Stop signals the migrator to stop after the current batch
func (m *Migrator) Stop() {
	close(m.stopChan)
	m.stopped = true
}

// Run executes the migration
func (m *Migrator) Run(ctx context.Context) error {
	startTime := time.Now()
	m.state.StartNewRun(string(m.config.Mode), m.config.BatchLimit)

	// Determine how many batches to run
	maxBatches := -1 // unlimited for continuous mode
	if m.config.Mode == RunModeBatches {
		maxBatches = m.config.BatchLimit
	}

	logDebug("[MIGRATION] Starting: mode=%s, batchLimit=%d, maxBatches=%d, cursor=%d, maxID=%d",
		m.config.Mode, m.config.BatchLimit, maxBatches, m.state.Progress.LastCursor, m.state.Source.MaxID)

	batchCount := 0
	cursor := m.state.Progress.LastCursor

	for {
		logDebug("[MIGRATION] Loop: batchCount=%d/%d, cursor=%d", batchCount, maxBatches, cursor)
		// Check for stop signal
		select {
		case <-m.stopChan:
			logDebug("[MIGRATION] Stop signal received")
			m.finalize(startTime)
			return nil
		case <-ctx.Done():
			logDebug("[MIGRATION] Context cancelled: %v", ctx.Err())
			m.finalize(startTime)
			return ctx.Err()
		default:
		}

		// Check if we've reached the batch limit
		if maxBatches > 0 && batchCount >= maxBatches {
			logDebug("[MIGRATION] Reached batch limit: %d >= %d", batchCount, maxBatches)
			break
		}

		// Check if migration is complete
		if cursor >= m.state.Source.MaxID {
			logDebug("[MIGRATION] Complete: cursor=%d >= maxID=%d", cursor, m.state.Source.MaxID)
			break
		}

		// Process one batch
		logDebug("[MIGRATION] Processing batch %d: cursor=%d, limit=%d", batchCount+1, cursor, m.config.BatchSize)
		processed, imported, skipped, lastID, err := m.processBatch(ctx, cursor)
		if err != nil {
			logDebug("[MIGRATION] ERROR: Batch failed: %v", err)
			m.finalize(startTime)
			return fmt.Errorf("batch processing failed: %w", err)
		}

		logDebug("[MIGRATION] Batch %d complete: processed=%d, imported=%d, skipped=%d, lastID=%d",
			batchCount+1, processed, imported, skipped, lastID)

		// Update state
		m.state.UpdateAfterBatch(lastID, processed, imported, skipped)
		if err := m.state.Save(); err != nil {
			// Log but don't fail - we can recover
			logDebug("Warning: failed to save state: %v", err)
		}

		cursor = lastID
		batchCount++

		// Call progress callback
		if m.onProgress != nil {
			m.onProgress(MigrationStats{
				BatchesCompleted: m.state.Batches.Completed,
				RowsProcessed:    m.state.Progress.ProcessedRows,
				RowsImported:     m.state.Progress.ImportedRows,
				RowsSkipped:      m.state.Progress.SkippedRows,
				CurrentCursor:    cursor,
				StartTime:        startTime,
				LastBatchTime:    time.Now(),
				TotalRows:        m.state.Source.TotalRows,
				ErrorLogPath:     m.errorLogger.Path(),
			})
		}
	}

	logDebug("[MIGRATION] Exiting loop: batchCount=%d, cursor=%d", batchCount, cursor)
	m.finalize(startTime)
	logDebug("[MIGRATION] Finalized successfully")
	return nil
}

// processBatch processes a single batch of rows
func (m *Migrator) processBatch(ctx context.Context, cursor int64) (processed, imported, skipped int, lastID int64, err error) {
	logDebug("[BATCH] Fetching from cursor=%d, limit=%d", cursor, m.config.BatchSize)

	// Fetch batch from MySQL
	rows, err := m.mysql.FetchBatch(ctx, m.config.SourceTable, m.config.SourceColumns, m.config.SourcePK, cursor, m.config.BatchSize)
	if err != nil {
		logDebug("[BATCH] ERROR: FetchBatch failed: %v", err)
		return 0, 0, 0, cursor, fmt.Errorf("failed to fetch batch: %w", err)
	}
	defer rows.Close()

	// Prepare data for insertion
	var insertRows [][]interface{}
	var rowIDs []int64

	for rows.Next() {
		// Create scan destinations
		// First column is always the PK
		numCols := len(m.config.SourceColumns) + 1 // +1 for PK
		if containsString(m.config.SourceColumns, m.config.SourcePK) {
			numCols = len(m.config.SourceColumns)
		}

		scanDest := make([]interface{}, numCols)
		for i := range scanDest {
			var v interface{}
			scanDest[i] = &v
		}

		if err := rows.Scan(scanDest...); err != nil {
			return processed, imported, skipped, lastID, fmt.Errorf("failed to scan row: %w", err)
		}

		// Extract PK value (always first)
		pkVal, ok := (*scanDest[0].(*interface{})).(int64)
		if !ok {
			// Try to convert from other numeric types
			switch v := (*scanDest[0].(*interface{})).(type) {
			case int:
				pkVal = int64(v)
			case int32:
				pkVal = int64(v)
			case uint64:
				pkVal = int64(v)
			case float64:
				pkVal = int64(v)
			default:
				return processed, imported, skipped, lastID, fmt.Errorf("unexpected PK type: %T", v)
			}
		}
		lastID = pkVal
		processed++

		// Transform and validate row data
		insertRow, err := m.transformRow(scanDest, pkVal)
		if err != nil {
			// Log error and skip this row; if the log write itself fails, stop —
			// the log is the only record of which rows were dropped.
			if logErr := m.errorLogger.Log(pkVal, err, fmt.Sprintf("%v", scanDest)); logErr != nil {
				return processed, imported, skipped, lastID, fmt.Errorf("error log write failed (cannot record skipped row %d): %w", pkVal, logErr)
			}
			skipped++
			continue
		}

		insertRows = append(insertRows, insertRow)
		rowIDs = append(rowIDs, pkVal)
	}

	if err := rows.Err(); err != nil {
		logDebug("[BATCH] ERROR: Row iteration error: %v", err)
		return processed, imported, skipped, lastID, fmt.Errorf("error iterating rows: %w", err)
	}

	logDebug("[BATCH] Fetched %d rows, inserting into %s", len(insertRows), m.config.TargetTable)

	// Insert into PostgreSQL
	if len(insertRows) > 0 {
		insertedCount, err := m.postgres.InsertBatch(ctx, m.config.TargetTable, m.config.TargetColumns, insertRows)
		if err != nil {
			logDebug("[BATCH] Bulk insert failed: %v, retrying %d rows individually", err, len(insertRows))
			imp, skp, retryErr := m.retryRowsIndividually(ctx, insertRows, rowIDs)
			imported += imp
			skipped += skp
			if retryErr != nil {
				return processed, imported, skipped, lastID, retryErr
			}
			logDebug("[BATCH] Individual inserts complete: imported=%d, skipped=%d", imported, skipped)
		} else {
			imported = insertedCount
			logDebug("[BATCH] Bulk insert successful: %d rows", insertedCount)
		}
	} else {
		logDebug("[BATCH] No rows to insert (empty batch)")
	}

	return processed, imported, skipped, lastID, nil
}

// retryRowsIndividually is the fallback when a bulk COPY fails: it re-inserts
// each row on its own so one bad row no longer loses the whole batch. Every row
// is accounted for — imported on success, or logged via the ErrorLogger and
// counted as skipped on failure — so the caller's processed total always equals
// imported + skipped. It stops early if ctx is cancelled (returning ctx.Err())
// instead of hammering a dead connection and flooding the log with one
// "context canceled" entry per remaining row.
func (m *Migrator) retryRowsIndividually(ctx context.Context, insertRows [][]interface{}, rowIDs []int64) (imported, skipped int, err error) {
	for i, row := range insertRows {
		if cerr := ctx.Err(); cerr != nil {
			return imported, skipped, cerr
		}
		if _, insertErr := m.postgres.InsertBatch(ctx, m.config.TargetTable, m.config.TargetColumns, [][]interface{}{row}); insertErr != nil {
			// The error log is now the only record of a dropped row; if we cannot
			// write it, fail the run rather than lose the row silently.
			if logErr := m.errorLogger.Log(rowIDs[i], insertErr, fmt.Sprintf("%v", row)); logErr != nil {
				return imported, skipped, fmt.Errorf("error log write failed (cannot record skipped row %d): %w", rowIDs[i], logErr)
			}
			skipped++
			continue
		}
		imported++
	}
	return imported, skipped, nil
}

// transformRow transforms a source row based on column mappings
func (m *Migrator) transformRow(scanDest []interface{}, pkVal int64) ([]interface{}, error) {
	result := make([]interface{}, len(m.config.TargetColumns))

	// Build a map of source values
	sourceValues := make(map[string]interface{})

	// First value is PK
	sourceValues[m.config.SourcePK] = pkVal

	// Remaining values map to source columns
	idx := 1
	for _, col := range m.config.SourceColumns {
		if col == m.config.SourcePK {
			continue // PK already handled
		}
		if idx < len(scanDest) {
			sourceValues[col] = *scanDest[idx].(*interface{})
			idx++
		}
	}

	// Apply mappings
	for i, targetCol := range m.config.TargetColumns {
		// Find the mapping for this target column
		var mapping *config.ColumnMapping
		for j := range m.config.Mapping {
			if m.config.Mapping[j].Target == targetCol {
				mapping = &m.config.Mapping[j]
				break
			}
		}

		if mapping == nil {
			// No mapping for this target column - use smart default
			result[i] = m.getDefaultValueForUnmappedColumn(targetCol)
			continue
		}

		sourceVal := sourceValues[mapping.Source]

		// Apply transform if needed
		transformedVal, err := m.applyTransform(sourceVal, mapping.Transform, pkVal)
		if err != nil {
			return nil, err
		}

		result[i] = transformedVal
	}

	return result, nil
}

// applyTransform applies a transformation to a value
func (m *Migrator) applyTransform(val interface{}, transform string, pkVal int64) (interface{}, error) {
	switch transform {
	case "text_to_jsonb", "TEXT_TO_JSONB":
		// Validate that the value is valid JSON
		var strVal string
		switch v := val.(type) {
		case string:
			strVal = v
		case []byte:
			strVal = string(v)
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("expected string for JSON transform, got %T", val)
		}

		// Validate JSON
		if !json.Valid([]byte(strVal)) {
			return nil, fmt.Errorf("invalid JSON")
		}

		return strVal, nil

	case "int_to_bool", "INT_TO_BOOL":
		switch v := val.(type) {
		case int64:
			return v != 0, nil
		case int32:
			return v != 0, nil
		case int:
			return v != 0, nil
		case bool:
			return v, nil
		case []byte:
			return len(v) > 0 && v[0] != '0', nil
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("int_to_bool: expected int/bool, got %T", val)
		}

	case "string_to_uuid", "STRING_TO_UUID":
		// MySQL returns CHAR/VARCHAR columns as []byte when scanning into interface{}.
		// pgx.CopyFrom cannot encode []byte into a PostgreSQL uuid column directly;
		// converting to a plain string lets pgx use the text protocol which PostgreSQL
		// accepts for uuid columns. Format validation is left to PostgreSQL — malformed
		// values will be caught per-row and logged via the ErrorLogger.
		switch v := val.(type) {
		case []byte:
			return string(v), nil
		case string:
			return v, nil
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("string_to_uuid: expected string/[]byte, got %T", val)
		}

	case "", "none":
		// No transform. Go's MySQL driver returns []byte for CHAR/VARCHAR/TEXT
		// columns when scanning into interface{}; pgx.CopyFrom binary-encodes a
		// []byte into a text column as bytea (`\x<hex>`), corrupting the value
		// (e.g. "Teresa" -> "\x546572657361"). Converting to string forces the
		// text protocol, which PostgreSQL accepts for text/timestamp/etc. (same
		// fix as string_to_uuid above). Non-[]byte values (int64, bool,
		// time.Time, nil) pass through untouched.
		if b, ok := val.([]byte); ok {
			return string(b), nil
		}
		return val, nil

	default:
		return nil, fmt.Errorf("unknown transform: %s", transform)
	}
}

// getDefaultValueForUnmappedColumn returns a safe default value for columns
// that aren't mapped from the source. This prevents NOT NULL constraint violations.
func (m *Migrator) getDefaultValueForUnmappedColumn(columnName string) interface{} {
	// Look up the column metadata
	colInfo, exists := m.targetColumns[columnName]
	if !exists {
		// Column not found in metadata - return nil
		return nil
	}

	// If column is nullable, NULL is fine
	if colInfo.IsNullable {
		return nil
	}

	// Column is NOT NULL - need a default value

	// If it has a database default, PostgreSQL will use it (return nil here)
	if colInfo.HasDefault {
		return nil
	}

	// No default defined - we need to provide one
	// Use type-based defaults to prevent constraint violations
	switch colInfo.DataType {
	case "json", "jsonb":
		// Empty JSON object
		return "{}"
	case "text", "varchar", "character varying":
		// Empty string
		return ""
	case "integer", "bigint", "smallint":
		// Zero
		return 0
	case "boolean":
		// False
		return false
	case "timestamp", "timestamp with time zone", "timestamp without time zone":
		// Current timestamp
		return time.Now()
	case "date":
		// Current date
		return time.Now()
	default:
		// Unknown type - return nil and let it fail with a clear error
		return nil
	}
}

// finalize cleans up and saves final state
func (m *Migrator) finalize(startTime time.Time) {
	m.state.EndRun(time.Since(startTime))
	m.state.Save()
	m.errorLogger.Close()
}

// GetState returns the current state
func (m *Migrator) GetState() *State {
	return m.state
}

// GetErrorLogger returns the error logger
func (m *Migrator) GetErrorLogger() *ErrorLogger {
	return m.errorLogger
}

// containsString checks if a slice contains a string
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
