package cmd

import (
	"flag"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ExecutePruneReplicas removes stale replica entries from the database.
func ExecutePruneReplicas(args []string) error {
	fs := flag.NewFlagSet("prune-replicas", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "Preview deletions without modifying database")
	olderThan := fs.Duration("older-than", 5*time.Minute, "Remove replicas with no heartbeat older than this duration")
	dbPath := fs.String("db", getEnv("NEBULAGC_DB_PATH", "./nebula.db"), "Path to SQLite database")
	verbose := fs.Bool("verbose", false, "Enable verbose output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Setup logger
	logConfig := zap.NewDevelopmentConfig()
	if !*verbose {
		logConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	logger, err := logConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	// Open database
	db, err := OpenDatabase(*dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	logger.Info("pruning stale replicas",
		zap.Duration("older_than", *olderThan),
		zap.Bool("dry_run", *dryRun),
	)

	// Calculate cutoff time
	cutoff := time.Now().Add(-*olderThan)

	// Find stale replicas
	query := `
		SELECT id, node_id, api_url, last_heartbeat, status
		FROM replicas
		WHERE last_heartbeat < ?
		ORDER BY last_heartbeat ASC
	`

	rows, err := db.Query(query, cutoff)
	if err != nil {
		return fmt.Errorf("failed to query stale replicas: %w", err)
	}
	defer rows.Close()

	type staleReplica struct {
		ID            string
		NodeID        string
		APIURL        string
		LastHeartbeat time.Time
		Status        string
	}

	var replicas []staleReplica
	for rows.Next() {
		var r staleReplica
		if err := rows.Scan(&r.ID, &r.NodeID, &r.APIURL, &r.LastHeartbeat, &r.Status); err != nil {
			return fmt.Errorf("failed to scan replica: %w", err)
		}
		replicas = append(replicas, r)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating replicas: %w", err)
	}

	if len(replicas) == 0 {
		logger.Info("no stale replicas found")
		return nil
	}

	logger.Info("found stale replicas", zap.Int("count", len(replicas)))

	// Display replicas to be deleted
	fmt.Printf("\nStale replicas (older than %s):\n", *olderThan)
	fmt.Println("=====================================")
	for _, r := range replicas {
		age := time.Since(r.LastHeartbeat)
		fmt.Printf("  ID:              %s\n", r.ID)
		fmt.Printf("  Node ID:         %s\n", r.NodeID)
		fmt.Printf("  API URL:         %s\n", r.APIURL)
		fmt.Printf("  Status:          %s\n", r.Status)
		fmt.Printf("  Last Heartbeat:  %s (%.0f seconds ago)\n", r.LastHeartbeat.Format(time.RFC3339), age.Seconds())
		fmt.Println("  ---")
	}

	if *dryRun {
		fmt.Printf("\n[DRY RUN] Would delete %d stale replica(s)\n", len(replicas))
		return nil
	}

	// Delete stale replicas
	deleteQuery := `DELETE FROM replicas WHERE last_heartbeat < ?`
	result, err := db.Exec(deleteQuery, cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete stale replicas: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	logger.Info("pruned stale replicas", zap.Int64("deleted", deleted))
	fmt.Printf("\n✓ Successfully deleted %d stale replica(s)\n", deleted)

	return nil
}
