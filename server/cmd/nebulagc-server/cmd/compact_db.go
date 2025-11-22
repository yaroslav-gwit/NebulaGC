package cmd

import (
	"flag"
	"fmt"

	"go.uber.org/zap"
)

// ExecuteCompactDB compacts the SQLite database to reclaim space.
func ExecuteCompactDB(args []string) error {
	fs := flag.NewFlagSet("compact-db", flag.ExitOnError)
	dbPath := fs.String("db", getEnv("NEBULAGC_DB_PATH", "./nebula.db"), "Path to SQLite database")
	analyze := fs.Bool("analyze", true, "Run ANALYZE after VACUUM")
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

	logger.Info("compacting database", zap.String("path", *dbPath))

	// Get database size before compaction
	var pageCount, pageSize int64
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return fmt.Errorf("failed to get page count: %w", err)
	}
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil {
		return fmt.Errorf("failed to get page size: %w", err)
	}

	sizeBefore := pageCount * pageSize
	fmt.Printf("Database size before: %.2f MB (%d pages x %d bytes)\n",
		float64(sizeBefore)/(1024*1024), pageCount, pageSize)

	// Run VACUUM
	logger.Info("running VACUUM...")
	fmt.Println("\nRunning VACUUM (this may take a while)...")
	if _, err := db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("VACUUM failed: %w", err)
	}

	// Get database size after compaction
	if err := db.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return fmt.Errorf("failed to get page count: %w", err)
	}

	sizeAfter := pageCount * pageSize
	saved := sizeBefore - sizeAfter
	percentSaved := (float64(saved) / float64(sizeBefore)) * 100

	fmt.Printf("\nDatabase size after:  %.2f MB (%d pages x %d bytes)\n",
		float64(sizeAfter)/(1024*1024), pageCount, pageSize)
	fmt.Printf("Space reclaimed:      %.2f MB (%.1f%%)\n",
		float64(saved)/(1024*1024), percentSaved)

	logger.Info("VACUUM completed",
		zap.Int64("size_before", sizeBefore),
		zap.Int64("size_after", sizeAfter),
		zap.Int64("saved", saved),
	)

	// Run ANALYZE if requested
	if *analyze {
		logger.Info("running ANALYZE...")
		fmt.Println("\nRunning ANALYZE to update query optimizer statistics...")
		if _, err := db.Exec("ANALYZE"); err != nil {
			return fmt.Errorf("ANALYZE failed: %w", err)
		}
		fmt.Println("✓ ANALYZE completed")
		logger.Info("ANALYZE completed")
	}

	// Show table statistics
	fmt.Println("\nTable Statistics:")
	fmt.Println("=====================================")

	tables := []string{"tenants", "clusters", "cluster_state", "nodes", "config_bundles", "replicas"}
	for _, table := range tables {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		if err := db.QueryRow(query).Scan(&count); err != nil {
			logger.Warn("failed to count table rows", zap.String("table", table), zap.Error(err))
			continue
		}
		fmt.Printf("  %-20s %d rows\n", table+":", count)
	}

	fmt.Println("\n✓ Database compaction completed successfully")
	return nil
}
