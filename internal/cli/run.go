// Package cli implements the non-interactive subcommands for pgpipe.
// The run subcommand reads a config file and executes a migration headlessly,
// printing progress to stdout and exiting with a non-zero code on fatal error.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/db"
	"github.com/RobertoGongora/pgpipe/internal/migration"
)

// RunMigration implements `pgpipe run [--config=<path>]`.
//
// When --config is omitted, it falls back to the default .pgpipe/config.yaml
// and .pgpipe/state.yaml paths, matching TUI behaviour.
// When --config is provided, the state file lives alongside the config file as
// .<name>.state.yaml so that 85 parallel configs do not collide.
func RunMigration(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	configPath := fs.String("config", "", "Path to the migration config YAML file (default: .pgpipe/config.yaml)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pgpipe run [--config=<path>]\n\n")
		fmt.Fprintf(os.Stderr, "Run a migration headlessly from a config file.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Load config
	// -------------------------------------------------------------------------
	var cfg *config.Config
	var statePath string
	var err error

	if *configPath == "" {
		// Default path — backward compatible with TUI
		cfg, err = config.Load()
		statePath = filepath.Join(config.ConfigDir, config.StateFile)
	} else {
		cfg, err = config.LoadFromPath(*configPath)
		statePath = migration.StatePathForConfig(*configPath)
	}
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Basic validation
	if cfg.Migration.Source.Table == "" {
		return fmt.Errorf("config is missing migration.source.table")
	}
	if cfg.Migration.Target.Table == "" {
		return fmt.Errorf("config is missing migration.target.table")
	}
	if len(cfg.Migration.Mapping) == 0 {
		return fmt.Errorf("config has no column mappings")
	}

	batchSize := cfg.Migration.Settings.BatchSize
	if batchSize <= 0 {
		batchSize = 5000
	}

	label := cfg.Migration.Source.Table
	fmt.Printf("[pgpipe] Starting migration: %s → %s (batch_size=%d)\n",
		cfg.Migration.Source.Table, cfg.Migration.Target.Table, batchSize)

	// -------------------------------------------------------------------------
	// Connect to databases
	// -------------------------------------------------------------------------
	ctx := context.Background()

	mysqlClient, err := db.NewMySQLClient(&cfg.MySQL)
	if err != nil {
		return fmt.Errorf("failed to create MySQL client: %w", err)
	}
	defer mysqlClient.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	if err := mysqlClient.Ping(pingCtx); err != nil {
		cancel()
		return fmt.Errorf("MySQL ping failed: %w", err)
	}
	cancel()

	pgClient, err := db.NewPostgresClient(&cfg.PostgreSQL)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL client: %w", err)
	}
	defer pgClient.Close()

	pingCtx, cancel = context.WithTimeout(ctx, 10*time.Second)
	if err := pgClient.Ping(pingCtx); err != nil {
		cancel()
		return fmt.Errorf("PostgreSQL ping failed: %w", err)
	}
	cancel()

	fmt.Printf("[pgpipe] Connected to MySQL and PostgreSQL\n")

	// -------------------------------------------------------------------------
	// Resolve source columns and primary key
	// -------------------------------------------------------------------------
	sourceColumns := cfg.Migration.Source.Columns
	pkColumn := cfg.Migration.Source.PrimaryKey

	// If no columns listed in config, fetch all from MySQL
	if len(sourceColumns) == 0 {
		cols, err := mysqlClient.GetColumns(ctx, cfg.Migration.Source.Table)
		if err != nil {
			return fmt.Errorf("failed to get source columns: %w", err)
		}
		for _, c := range cols {
			sourceColumns = append(sourceColumns, c.Name)
			if c.IsPrimaryKey && pkColumn == "" {
				pkColumn = c.Name
			}
		}
	}

	// If PK still not known, ask MySQL
	if pkColumn == "" {
		pkColumn, err = mysqlClient.GetPrimaryKey(ctx, cfg.Migration.Source.Table)
		if err != nil {
			return fmt.Errorf("failed to determine primary key: %w", err)
		}
	}

	// Build target column list from mappings (only mapped columns)
	var targetColumns []string
	for _, m := range cfg.Migration.Mapping {
		if m.Target != "" {
			targetColumns = append(targetColumns, m.Target)
		}
	}
	if len(targetColumns) == 0 {
		return fmt.Errorf("no mapped target columns found in config")
	}

	// -------------------------------------------------------------------------
	// Load or create state
	// -------------------------------------------------------------------------
	var state *migration.State

	existingState, err := migration.LoadStateFromPath(statePath)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	configHash := cfg.Hash()

	if existingState != nil {
		if existingState.ConfigHash != configHash {
			fmt.Printf("[pgpipe] WARNING: config has changed since last run (hash mismatch). Starting fresh.\n")
			existingState = nil
		} else if existingState.IsComplete() {
			fmt.Printf("[pgpipe] Migration already complete (%d rows processed). Use a fresh state to re-run.\n",
				existingState.Progress.ProcessedRows)
			return nil
		} else {
			state = existingState
			fmt.Printf("[pgpipe] Resuming from cursor %d (%d rows already processed)\n",
				state.Progress.LastCursor, state.Progress.ProcessedRows)
		}
	}

	if state == nil {
		state = migration.NewState(configHash)
		state.SetStatePath(statePath)

		// Populate source stats
		totalRows, err := mysqlClient.GetTableRowCount(ctx, cfg.Migration.Source.Table)
		if err != nil {
			return fmt.Errorf("failed to count source rows: %w", err)
		}
		minID, maxID, err := mysqlClient.GetMinMaxID(ctx, cfg.Migration.Source.Table, pkColumn)
		if err != nil {
			return fmt.Errorf("failed to get ID range: %w", err)
		}

		state.Source = migration.SourceState{
			Table:      cfg.Migration.Source.Table,
			TotalRows:  totalRows,
			PrimaryKey: pkColumn,
			MinID:      minID,
			MaxID:      maxID,
		}
		state.Batches.Size = batchSize

		fmt.Printf("[pgpipe] Source: %d rows (id %d..%d)\n", totalRows, minID, maxID)
	} else {
		// Ensure the custom state path is set on the resumed state so Save()
		// writes to the right place.
		state.SetStatePath(statePath)
	}

	// -------------------------------------------------------------------------
	// Create migrator and run
	// -------------------------------------------------------------------------
	migCfg := migration.MigrationConfig{
		SourceTable:   cfg.Migration.Source.Table,
		TargetTable:   cfg.Migration.Target.Table,
		SourcePK:      pkColumn,
		SourceColumns: sourceColumns,
		TargetColumns: targetColumns,
		Mapping:       cfg.Migration.Mapping,
		BatchSize:     batchSize,
		Mode:          migration.RunModeContinuous, // CLI always runs to completion
	}

	migrator, err := migration.NewMigrator(mysqlClient, pgClient, migCfg, state)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Progress callback: print one line per batch
	startTime := time.Now()
	migrator.SetProgressCallback(func(stats migration.MigrationStats) {
		pct := 0.0
		if stats.TotalRows > 0 {
			pct = float64(stats.RowsProcessed) / float64(stats.TotalRows) * 100
		}
		elapsed := time.Since(startTime).Round(time.Second)
		fmt.Printf("[pgpipe] %s: batch %d | %s/%s (%.1f%%) | imported=%s skipped=%s | elapsed=%s\n",
			label,
			stats.BatchesCompleted,
			formatInt(stats.RowsProcessed),
			formatInt(stats.TotalRows),
			pct,
			formatInt(stats.RowsImported),
			formatInt(stats.RowsSkipped),
			elapsed,
		)
	})

	runErr := migrator.Run(ctx)

	// -------------------------------------------------------------------------
	// Print summary
	// -------------------------------------------------------------------------
	finalState := migrator.GetState()
	errLogger := migrator.GetErrorLogger()
	elapsed := time.Since(startTime).Round(time.Millisecond)

	fmt.Printf("\n[pgpipe] Migration complete: %s\n", label)
	fmt.Printf("  Processed : %s rows\n", formatInt(finalState.Progress.ProcessedRows))
	fmt.Printf("  Imported  : %s rows\n", formatInt(finalState.Progress.ImportedRows))
	fmt.Printf("  Skipped   : %s rows\n", formatInt(finalState.Progress.SkippedRows))
	fmt.Printf("  Duration  : %s\n", elapsed)
	if errLogger.Count() > 0 {
		fmt.Printf("  Errors    : %d (see %s)\n", errLogger.Count(), errLogger.Path())
	}

	// Reconcile and fail loudly (non-zero exit) if the load is not clean — a
	// "successful" run that imported fewer rows than it processed is exactly the
	// failure mode this tool must never report as success. Run this even when
	// runErr != nil so a partial/cancelled run still surfaces its gap; only
	// promote the reconcile error to the exit status if there isn't already a
	// fatal error to report.
	p := finalState.Progress
	if rerr := reconcile(p.ProcessedRows, p.ImportedRows, p.SkippedRows); rerr != nil {
		fmt.Printf("  Reconcile : FAIL — %v\n", rerr)
		if runErr == nil {
			runErr = rerr
		}
	} else {
		fmt.Printf("  Reconcile : OK — imported == processed (%s)\n", formatInt(p.ImportedRows))
	}

	return runErr
}

// reconcile verifies a clean load. It returns nil ONLY when imported ==
// processed with zero skips. Otherwise it returns a non-nil error:
//   - an unaccounted gap (processed != imported + skipped) means rows vanished
//     silently — the failure mode this tool was hardened against;
//   - any skipped rows mean rows were rejected and the load is incomplete.
func reconcile(processed, imported, skipped int64) error {
	if gap := processed - imported - skipped; gap != 0 {
		return fmt.Errorf("reconciliation failed: processed=%d imported=%d skipped=%d (gap=%d) — possible silent loss",
			processed, imported, skipped, gap)
	}
	if skipped > 0 {
		return fmt.Errorf("incomplete load: %d of %d rows skipped (see error log)", skipped, processed)
	}
	return nil
}

// formatInt formats an int64 with comma separators for readability.
func formatInt(n int64) string {
	if n < 0 {
		return "-" + formatInt(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	return formatInt(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}
