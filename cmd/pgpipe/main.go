package main

import (
	"fmt"
	"os"

	"github.com/pgpipe/pgpipe/internal/cli"
	"github.com/pgpipe/pgpipe/internal/tui"
)

const usage = `pgpipe — MySQL to PostgreSQL migration tool

Usage:
  pgpipe                              Launch the interactive TUI wizard
  pgpipe run [--config=<path>]        Run a migration headlessly from a config file
  pgpipe generate-configs [flags]     Generate per-table config files from live schemas

Run 'pgpipe <subcommand> --help' for subcommand usage.
`

func main() {
	if len(os.Args) < 2 {
		// No subcommand — launch TUI (default/backward-compatible behaviour)
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch os.Args[1] {
	case "run":
		if err := cli.RunMigration(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "generate-configs":
		if err := cli.GenerateConfigs(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "--help", "-h", "help":
		fmt.Print(usage)

	default:
		// Unknown argument — fall through to TUI so existing integrations are not broken
		if err := tui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
