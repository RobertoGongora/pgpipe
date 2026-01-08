package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pgpipe/pgpipe/internal/config"
	"github.com/pgpipe/pgpipe/internal/db"
)

// RunMode defines how the migration should run
type RunMode string

const (
	RunModeContinuous RunMode = "continuous"
	RunModeBatches    RunMode = "batches"
)

// MigrationConfig holds the configuration for a migration run
type MigrationConfig struct {
	SourceTable    string
	TargetTable    string
	SourcePK       string
	SourceColumns  []string
	TargetColumns  []string
	Mapping        []config.ColumnMapping
	BatchSize      int
	Mode           RunMode
	BatchLimit     int // Only used when Mode == RunModeBatches
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
	mysql       *db.MySQLClient
	postgres    *db.PostgresClient
	config      MigrationConfig
	state       *State
	errorLogger *ErrorLogger
	onProgress  ProgressCallback
	stopChan    chan struct{}
	stopped     bool
}

// NewMigrator creates a new migrator instance
func NewMigrator(
	mysql *db.MySQLClient,
	postgres *db.PostgresClient,
	cfg MigrationConfig,
	state *State,
) (*Migrator, error) {
	errorLogger, err := NewErrorLogger(state.Session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create error logger: %w", err)
	}

	return &Migrator{
		mysql:       mysql,
		postgres:    postgres,
		config:      cfg,
		state:       state,
		errorLogger: errorLogger,
		stopChan:    make(chan struct{}),
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

	batchCount := 0
	cursor := m.state.Progress.LastCursor

	for {
		// Check for stop signal
		select {
		case <-m.stopChan:
			m.finalize(startTime)
			return nil
		case <-ctx.Done():
			m.finalize(startTime)
			return ctx.Err()
		default:
		}

		// Check if we've reached the batch limit
		if maxBatches > 0 && batchCount >= maxBatches {
			break
		}

		// Check if migration is complete
		if cursor >= m.state.Source.MaxID {
			break
		}

		// Process one batch
		processed, imported, skipped, lastID, err := m.processBatch(ctx, cursor)
		if err != nil {
			m.finalize(startTime)
			return fmt.Errorf("batch processing failed: %w", err)
		}

		// Update state
		m.state.UpdateAfterBatch(lastID, processed, imported, skipped)
		if err := m.state.Save(); err != nil {
			// Log but don't fail - we can recover
			fmt.Printf("Warning: failed to save state: %v\n", err)
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

	m.finalize(startTime)
	return nil
}

// processBatch processes a single batch of rows
func (m *Migrator) processBatch(ctx context.Context, cursor int64) (processed, imported, skipped int, lastID int64, err error) {
	// Fetch batch from MySQL
	rows, err := m.mysql.FetchBatch(ctx, m.config.SourceTable, m.config.SourceColumns, m.config.SourcePK, cursor, m.config.BatchSize)
	if err != nil {
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
			// Log error and skip this row
			m.errorLogger.Log(pkVal, err, fmt.Sprintf("%v", scanDest))
			skipped++
			continue
		}

		insertRows = append(insertRows, insertRow)
		rowIDs = append(rowIDs, pkVal)
	}

	if err := rows.Err(); err != nil {
		return processed, imported, skipped, lastID, fmt.Errorf("error iterating rows: %w", err)
	}

	// Insert into PostgreSQL
	if len(insertRows) > 0 {
		insertedCount, err := m.postgres.InsertBatch(ctx, m.config.TargetTable, m.config.TargetColumns, insertRows)
		if err != nil {
			// Try individual inserts and log failures
			for i, row := range insertRows {
				_, insertErr := m.postgres.InsertBatch(ctx, m.config.TargetTable, m.config.TargetColumns, [][]interface{}{row})
				if insertErr != nil {
					m.errorLogger.Log(rowIDs[i], insertErr, fmt.Sprintf("%v", row))
					skipped++
				} else {
					imported++
				}
			}
		} else {
			imported = insertedCount
		}
	}

	return processed, imported, skipped, lastID, nil
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
			result[i] = nil
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

	case "", "none":
		// No transform, pass through
		return val, nil

	default:
		return nil, fmt.Errorf("unknown transform: %s", transform)
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
