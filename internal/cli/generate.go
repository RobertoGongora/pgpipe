package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/RobertoGongora/pgpipe/internal/config"
	"github.com/RobertoGongora/pgpipe/internal/db"
)

// GenerateConfigs implements `pgpipe generate-configs --output-dir=<dir>`.
//
// It introspects the MySQL source database and the PostgreSQL target database,
// then writes one config YAML per MySQL table into the output directory.
// Transforms are auto-detected using the same rules as the TUI.
func GenerateConfigs(args []string) error {
	fs := flag.NewFlagSet("generate-configs", flag.ContinueOnError)
	outputDir := fs.String("output-dir", "", "Directory to write generated config files (required)")
	skipList := fs.String("skip", "", "Comma-separated list of table names to skip")
	force := fs.Bool("force", false, "Overwrite existing config files")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: pgpipe generate-configs --output-dir=<dir> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Generate per-table migration config files from live schema introspection.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *outputDir == "" {
		fs.Usage()
		return fmt.Errorf("--output-dir is required")
	}

	// Build skip set
	skipSet := make(map[string]bool)
	if *skipList != "" {
		for _, t := range strings.Split(*skipList, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				skipSet[t] = true
			}
		}
	}

	// -------------------------------------------------------------------------
	// Connect to databases using env var defaults
	// -------------------------------------------------------------------------
	cfg := config.NewDefaultConfig()
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

	fmt.Printf("[pgpipe] Connected to MySQL (%s) and PostgreSQL (%s)\n",
		cfg.MySQL.Database, cfg.PostgreSQL.Database)

	// -------------------------------------------------------------------------
	// List tables
	// -------------------------------------------------------------------------
	mysqlTables, err := mysqlClient.GetTables(ctx)
	if err != nil {
		return fmt.Errorf("failed to list MySQL tables: %w", err)
	}

	pgTables, err := pgClient.GetTables(ctx)
	if err != nil {
		return fmt.Errorf("failed to list PostgreSQL tables: %w", err)
	}

	// Build PG table lookup: bare name → schema.table
	// PG returns "public.tablename" format; we index by bare name for matching.
	pgTableMap := make(map[string]string)
	for _, t := range pgTables {
		bare := t.Name
		if idx := strings.Index(t.Name, "."); idx >= 0 {
			bare = t.Name[idx+1:]
		}
		pgTableMap[bare] = t.Name
	}

	// -------------------------------------------------------------------------
	// Ensure output directory exists
	// -------------------------------------------------------------------------
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// -------------------------------------------------------------------------
	// Generate one config per MySQL table
	// -------------------------------------------------------------------------
	var (
		nGenerated int
		nExisting  int
		nSkipList  int
		nNoMatch   int
	)

	for _, mysqlTable := range mysqlTables {
		tableName := mysqlTable.Name

		// Skip list
		if skipSet[tableName] {
			nSkipList++
			fmt.Printf("[pgpipe] skip (skip list): %s\n", tableName)
			continue
		}

		outFile := filepath.Join(*outputDir, tableName+".yaml")

		// Already exists?
		if !*force {
			if _, err := os.Stat(outFile); err == nil {
				nExisting++
				fmt.Printf("[pgpipe] skip (exists): %s\n", outFile)
				continue
			}
		}

		// Find matching PG table
		pgTableFull, ok := pgTableMap[tableName]
		if !ok {
			nNoMatch++
			fmt.Printf("[pgpipe] skip (no PG match): %s\n", tableName)
			continue
		}

		// Fetch columns for both sides
		mysqlCols, err := mysqlClient.GetColumns(ctx, tableName)
		if err != nil {
			fmt.Printf("[pgpipe] WARNING: failed to get MySQL columns for %s: %v\n", tableName, err)
			nNoMatch++
			continue
		}

		pgCols, err := pgClient.GetColumns(ctx, pgTableFull)
		if err != nil {
			fmt.Printf("[pgpipe] WARNING: failed to get PostgreSQL columns for %s: %v\n", pgTableFull, err)
			nNoMatch++
			continue
		}

		// Build PG column lookup
		pgColMap := make(map[string]db.ColumnInfo)
		for _, c := range pgCols {
			pgColMap[c.Name] = c
		}

		// Determine PK
		pkColumn := ""
		for _, c := range mysqlCols {
			if c.IsPrimaryKey {
				pkColumn = c.Name
				break
			}
		}

		// Build source column list and mappings
		var sourceColumns []string
		var mappings []config.ColumnMapping

		for _, srcCol := range mysqlCols {
			sourceColumns = append(sourceColumns, srcCol.Name)

			mapping := config.ColumnMapping{
				Source: srcCol.Name,
			}

			if pgCol, ok := pgColMap[srcCol.Name]; ok {
				mapping.Target = pgCol.Name
				mapping.Transform = db.DetectTransform(srcCol.DataType, pgCol.DataType)
			}
			// If no PG match, leave Target empty — user can fill in manually

			mappings = append(mappings, mapping)
		}

		// Build migration config (connection fields intentionally left empty —
		// they are filled at runtime from env vars via config.LoadFromPath).
		migCfg := config.MigrationConfig{
			Source: config.SourceConfig{
				Table:      tableName,
				PrimaryKey: pkColumn,
				Columns:    sourceColumns,
			},
			Target: config.TargetConfig{
				Table: pgTableFull,
			},
			Mapping: mappings,
			Settings: config.SettingsConfig{
				BatchSize: 5000,
			},
		}

		// Wrap in a top-level struct with only the migration key so the file is
		// migration-only — connections always come from env vars.
		type migrationOnlyConfig struct {
			Migration config.MigrationConfig `yaml:"migration"`
		}

		yamlData, err := yaml.Marshal(migrationOnlyConfig{Migration: migCfg})
		if err != nil {
			return fmt.Errorf("failed to marshal config for %s: %w", tableName, err)
		}

		if err := os.WriteFile(outFile, yamlData, 0644); err != nil {
			return fmt.Errorf("failed to write config for %s: %w", tableName, err)
		}

		nGenerated++
		fmt.Printf("[pgpipe] generated: %s\n", outFile)
	}

	// -------------------------------------------------------------------------
	// Summary
	// -------------------------------------------------------------------------
	fmt.Printf("\n[pgpipe] Done.\n")
	fmt.Printf("  Generated : %d\n", nGenerated)
	fmt.Printf("  Skipped (exists)    : %d\n", nExisting)
	fmt.Printf("  Skipped (skip list) : %d\n", nSkipList)
	fmt.Printf("  Skipped (no PG match) : %d\n", nNoMatch)

	return nil
}
