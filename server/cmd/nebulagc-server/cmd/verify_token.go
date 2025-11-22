package cmd

import (
	"flag"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// ExecuteVerifyToken verifies if a token matches the stored hash.
func ExecuteVerifyToken(args []string) error {
	fs := flag.NewFlagSet("verify-token", flag.ExitOnError)
	nodeID := fs.String("node-id", "", "Node ID to verify token for")
	clusterID := fs.String("cluster-id", "", "Cluster ID to verify token for")
	token := fs.String("token", "", "Token to verify (required)")
	dbPath := fs.String("db", getEnv("NEBULAGC_DB_PATH", "./nebula.db"), "Path to SQLite database")
	verbose := fs.Bool("verbose", false, "Enable verbose output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate arguments
	if *token == "" {
		return fmt.Errorf("--token is required")
	}

	if *nodeID == "" && *clusterID == "" {
		return fmt.Errorf("either --node-id or --cluster-id must be specified")
	}

	if *nodeID != "" && *clusterID != "" {
		return fmt.Errorf("cannot specify both --node-id and --cluster-id")
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

	var hash string
	var entityID string
	var entityType string

	if *nodeID != "" {
		// Verify node token
		entityType = "node"
		entityID = *nodeID
		logger.Info("verifying node token", zap.String("node_id", *nodeID))

		query := `SELECT token_hash FROM nodes WHERE id = ?`
		if err := db.QueryRow(query, *nodeID).Scan(&hash); err != nil {
			return fmt.Errorf("node not found or error querying: %w", err)
		}
	} else {
		// Verify cluster token
		entityType = "cluster"
		entityID = *clusterID
		logger.Info("verifying cluster token", zap.String("cluster_id", *clusterID))

		// Note: Cluster tokens would be stored differently - this is a placeholder
		// In the current schema, cluster tokens aren't stored separately
		return fmt.Errorf("cluster token verification not yet implemented (cluster tokens are managed per-node)")
	}

	fmt.Printf("\nVerifying %s token for: %s\n", entityType, entityID)
	fmt.Println("=====================================")

	// Compare token with hash
	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(*token))
	if err != nil {
		fmt.Printf("\n✗ Token verification FAILED\n")
		fmt.Printf("  Reason: Token does not match stored hash\n")
		logger.Error("token verification failed", zap.String(entityType+"_id", entityID))
		return fmt.Errorf("token verification failed")
	}

	fmt.Printf("\n✓ Token verification SUCCESSFUL\n")
	fmt.Printf("  The provided token matches the stored hash\n")

	if *verbose {
		fmt.Printf("\nToken Details:\n")
		fmt.Printf("  Length: %d characters\n", len(*token))
		fmt.Printf("  Hash:   %s...\n", hash[:20])
	}

	logger.Info("token verified successfully", zap.String(entityType+"_id", entityID))
	return nil
}
