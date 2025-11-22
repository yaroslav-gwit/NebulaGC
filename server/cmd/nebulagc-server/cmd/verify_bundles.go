package cmd

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"

	"go.uber.org/zap"
)

// ExecuteVerifyBundles verifies the integrity of configuration bundles.
func ExecuteVerifyBundles(args []string) error {
	fs := flag.NewFlagSet("verify-bundles", flag.ExitOnError)
	clusterID := fs.String("cluster-id", "", "Verify bundles for specific cluster (default: all clusters)")
	dbPath := fs.String("db", getEnv("NEBULAGC_DB_PATH", "./nebula.db"), "Path to SQLite database")
	verbose := fs.Bool("verbose", false, "Enable verbose output")
	fix := fs.Bool("fix", false, "Attempt to fix corrupted bundles (not implemented)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *fix {
		return fmt.Errorf("--fix mode not yet implemented")
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

	logger.Info("verifying bundles", zap.String("cluster_id", *clusterID))

	// Query bundles
	var query string
	var queryArgs []interface{}

	if *clusterID != "" {
		query = `
			SELECT id, cluster_id, version, config_data, activated_at
			FROM config_bundles
			WHERE cluster_id = ?
			ORDER BY version DESC
		`
		queryArgs = []interface{}{*clusterID}
	} else {
		query = `
			SELECT id, cluster_id, version, config_data, activated_at
			FROM config_bundles
			ORDER BY cluster_id, version DESC
		`
	}

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		return fmt.Errorf("failed to query bundles: %w", err)
	}
	defer rows.Close()

	type bundleInfo struct {
		ID          string
		ClusterID   string
		Version     int
		ConfigData  []byte
		ActivatedAt *string
	}

	var bundles []bundleInfo
	for rows.Next() {
		var b bundleInfo
		if err := rows.Scan(&b.ID, &b.ClusterID, &b.Version, &b.ConfigData, &b.ActivatedAt); err != nil {
			return fmt.Errorf("failed to scan bundle: %w", err)
		}
		bundles = append(bundles, b)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating bundles: %w", err)
	}

	if len(bundles) == 0 {
		logger.Info("no bundles found")
		return nil
	}

	logger.Info("found bundles", zap.Int("count", len(bundles)))

	// Verify each bundle
	valid := 0
	invalid := 0
	fmt.Printf("\nVerifying %d bundle(s):\n", len(bundles))
	fmt.Println("=====================================")

	for _, b := range bundles {
		fmt.Printf("\nBundle: %s (cluster: %s, version: %d)\n", b.ID, b.ClusterID, b.Version)
		fmt.Printf("  Size: %d bytes\n", len(b.ConfigData))

		if b.ActivatedAt != nil {
			fmt.Printf("  Status: Active (activated: %s)\n", *b.ActivatedAt)
		} else {
			fmt.Printf("  Status: Inactive\n")
		}

		// Verify tar.gz format
		if err := verifyTarGz(b.ConfigData); err != nil {
			fmt.Printf("  ✗ INVALID: %v\n", err)
			invalid++
			continue
		}

		// Check required files
		requiredFiles := []string{"config.yml"}
		missing, err := checkRequiredFiles(b.ConfigData, requiredFiles)
		if err != nil {
			fmt.Printf("  ✗ INVALID: %v\n", err)
			invalid++
			continue
		}

		if len(missing) > 0 {
			fmt.Printf("  ✗ INVALID: missing required files: %v\n", missing)
			invalid++
			continue
		}

		fmt.Printf("  ✓ Valid\n")
		valid++
	}

	fmt.Printf("\n=====================================\n")
	fmt.Printf("Summary: %d valid, %d invalid\n", valid, invalid)

	if invalid > 0 {
		return fmt.Errorf("found %d invalid bundle(s)", invalid)
	}

	logger.Info("all bundles verified successfully")
	return nil
}

// verifyTarGz verifies that data is a valid gzip-compressed tar archive.
func verifyTarGz(data []byte) error {
	// Create gzip reader
	gzReader, err := gzip.NewReader(io.NopCloser(&byteReader{data: data}))
	if err != nil {
		return fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Try to read at least one header
	_, err = tarReader.Next()
	if err != nil && err != io.EOF {
		return fmt.Errorf("invalid tar: %w", err)
	}

	return nil
}

// checkRequiredFiles checks if required files are present in the tar archive.
func checkRequiredFiles(data []byte, required []string) ([]string, error) {
	// Create gzip reader
	gzReader, err := gzip.NewReader(io.NopCloser(&byteReader{data: data}))
	if err != nil {
		return nil, fmt.Errorf("invalid gzip: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Track found files
	found := make(map[string]bool)
	for _, f := range required {
		found[f] = false
	}

	// Scan archive
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar read error: %w", err)
		}

		// Check if this is a required file
		for _, f := range required {
			if header.Name == f {
				found[f] = true
			}
		}
	}

	// Find missing files
	var missing []string
	for _, f := range required {
		if !found[f] {
			missing = append(missing, f)
		}
	}

	return missing, nil
}

// byteReader wraps a byte slice to implement io.Reader.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
