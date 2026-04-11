// Package main is the entry point for the OpenScanner migration runner.
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/openscanner/openscanner/internal/config"
	"github.com/openscanner/openscanner/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: migrate <up|down>")
		os.Exit(1)
	}

	direction := args[len(args)-1]
	switch direction {
	case "up":
		slog.Info("running migrations up", "db_file", cfg.DBFile)
		sqlDB, err := db.Open(cfg.DBFile)
		if err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		sqlDB.Close()
		slog.Info("migrations applied successfully")
	case "down":
		fmt.Fprintln(os.Stderr, "down migrations not yet supported")
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown migration direction %q (expected 'up' or 'down')\n", direction)
		os.Exit(1)
	}
}
