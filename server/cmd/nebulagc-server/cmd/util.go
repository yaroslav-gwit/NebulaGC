// Package cmd provides CLI commands for nebulagc-server.
package cmd

import (
	"database/sql"
	"fmt"
	"os"

	"go.uber.org/zap"
	_ "modernc.org/sqlite"
)

// UtilCommand represents the base utility command.
type UtilCommand struct {
	DB     *sql.DB
	Logger *zap.Logger
}

// OpenDatabase opens a connection to the SQLite database.
func OpenDatabase(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_timeout=5000&_foreign_keys=on", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// ExecuteUtil runs a utility command with the given arguments.
func ExecuteUtil(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("util command requires a subcommand\n\nAvailable subcommands:\n  prune-replicas    Remove stale replica entries\n  verify-bundles    Verify bundle integrity\n  compact-db        Compact and optimize database\n  check-lighthouses Check lighthouse process health\n  verify-token      Verify token authentication")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "prune-replicas":
		return ExecutePruneReplicas(subArgs)
	case "verify-bundles":
		return ExecuteVerifyBundles(subArgs)
	case "compact-db":
		return ExecuteCompactDB(subArgs)
	case "check-lighthouses":
		return ExecuteCheckLighthouses(subArgs)
	case "verify-token":
		return ExecuteVerifyToken(subArgs)
	default:
		return fmt.Errorf("unknown util subcommand: %s", subcommand)
	}
}

// getEnv retrieves an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
