package cmd

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// ExecuteCheckLighthouses checks the health of lighthouse processes.
func ExecuteCheckLighthouses(args []string) error {
	fs := flag.NewFlagSet("check-lighthouses", flag.ExitOnError)
	clusterID := fs.String("cluster-id", "", "Check lighthouses for specific cluster (default: all clusters)")
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

	logger.Info("checking lighthouse processes", zap.String("cluster_id", *clusterID))

	// Query clusters
	var query string
	var queryArgs []interface{}

	if *clusterID != "" {
		query = `
			SELECT c.id, c.name, cs.version, cs.node_count
			FROM clusters c
			JOIN cluster_state cs ON c.id = cs.cluster_id
			WHERE c.id = ?
		`
		queryArgs = []interface{}{*clusterID}
	} else {
		query = `
			SELECT c.id, c.name, cs.version, cs.node_count
			FROM clusters c
			JOIN cluster_state cs ON c.id = cs.cluster_id
			ORDER BY c.name
		`
	}

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return fmt.Errorf("failed to query clusters: %w", err)
	}
	defer rows.Close()

	type clusterInfo struct {
		ID        string
		Name      string
		Version   int
		NodeCount int
	}

	var clusters []clusterInfo
	for rows.Next() {
		var c clusterInfo
		if err := rows.Scan(&c.ID, &c.Name, &c.Version, &c.NodeCount); err != nil {
			return fmt.Errorf("failed to scan cluster: %w", err)
		}
		clusters = append(clusters, c)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating clusters: %w", err)
	}

	if len(clusters) == 0 {
		logger.Info("no clusters found")
		return nil
	}

	logger.Info("found clusters", zap.Int("count", len(clusters)))

	// Check lighthouse processes
	fmt.Printf("\nChecking lighthouse processes for %d cluster(s):\n", len(clusters))
	fmt.Println("=====================================")

	totalProcesses := 0
	runningProcesses := 0
	stoppedProcesses := 0

	for _, c := range clusters {
		fmt.Printf("\nCluster: %s (%s)\n", c.Name, c.ID)
		fmt.Printf("  Config Version: %d\n", c.Version)
		fmt.Printf("  Node Count:     %d\n", c.NodeCount)

		// Query replicas (instances running lighthouses)
		replicaQuery := `
			SELECT id, node_id, api_url, status
			FROM replicas
			WHERE status = 'active'
			ORDER BY id
		`

		replicaRows, err := db.Query(replicaQuery)
		if err != nil {
			return fmt.Errorf("failed to query replicas: %w", err)
		}

		type replicaInfo struct {
			ID     string
			NodeID string
			APIURL string
			Status string
		}

		var replicas []replicaInfo
		for replicaRows.Next() {
			var r replicaInfo
			if err := replicaRows.Scan(&r.ID, &r.NodeID, &r.APIURL, &r.Status); err != nil {
				replicaRows.Close()
				return fmt.Errorf("failed to scan replica: %w", err)
			}
			replicas = append(replicas, r)
		}
		replicaRows.Close()

		if len(replicas) == 0 {
			fmt.Printf("  No active replicas found (lighthouses not running)\n")
			continue
		}

		fmt.Printf("\n  Active Replicas: %d\n", len(replicas))
		fmt.Println("  ---")

		// Check each replica's lighthouse process
		for _, r := range replicas {
			totalProcesses++

			// Check if lighthouse process is running for this cluster
			// This is a simplified check - in production you'd query the actual process
			processName := fmt.Sprintf("lighthouse-%s-%s", c.ID, r.NodeID)
			running := checkProcessRunning(processName)

			status := "✓ Running"
			if !running {
				status = "✗ Stopped"
				stoppedProcesses++
			} else {
				runningProcesses++
			}

			fmt.Printf("    Replica: %s (%s)\n", r.NodeID, r.ID)
			fmt.Printf("      API URL: %s\n", r.APIURL)
			fmt.Printf("      Status:  %s\n", status)

			if *verbose && running {
				// In a real implementation, you'd get actual process info
				fmt.Printf("      Process: %s\n", processName)
			}
		}
	}

	fmt.Printf("\n=====================================\n")
	fmt.Printf("Summary:\n")
	fmt.Printf("  Total lighthouse processes:   %d\n", totalProcesses)
	fmt.Printf("  Running:                      %d\n", runningProcesses)
	fmt.Printf("  Stopped:                      %d\n", stoppedProcesses)

	if stoppedProcesses > 0 {
		fmt.Printf("\n⚠ Warning: %d lighthouse process(es) are not running\n", stoppedProcesses)
		return fmt.Errorf("found %d stopped lighthouse process(es)", stoppedProcesses)
	}

	fmt.Println("\n✓ All lighthouse processes are running")
	logger.Info("all lighthouses healthy")
	return nil
}

// checkProcessRunning checks if a process with the given name is running.
// This is a simplified implementation - in production you'd check actual PIDs.
func checkProcessRunning(name string) bool {
	// Try to use pgrep if available
	cmd := exec.Command("pgrep", "-f", name)
	output, err := cmd.Output()
	if err != nil {
		// pgrep not available or process not found
		return false
	}

	// Check if we got any PIDs back
	return len(strings.TrimSpace(string(output))) > 0
}

// Note: This is a simplified check. In a real implementation, you would:
// 1. Query the lighthouse manager's state from the database or API
// 2. Check actual PID files or process managers
// 3. Verify process responsiveness (not just existence)
// 4. Check log files for errors
// 5. Validate Nebula process connectivity
