package service

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
	"nebulagc.io/pkg/bundle"
)

// BundleService provides operations for managing config bundles.
type BundleService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewBundleService creates a new bundle service.
//
// Parameters:
//   - db: Database connection
//   - logger: Zap logger for structured logging
//
// Returns:
//   - Configured BundleService
func NewBundleService(db *sql.DB, logger *zap.Logger) *BundleService {
	return &BundleService{
		db:     db,
		logger: logger,
	}
}

// Upload validates and stores a new config bundle for a cluster.
//
// This function:
// 1. Validates the bundle using bundle.Validate()
// 2. Increments the cluster's config_version
// 3. Stores the bundle in config_bundles table
//
// Parameters:
//   - clusterID: The cluster ID
//   - data: The bundle data (tar.gz)
//
// Returns:
//   - int64: The new version number
//   - error: Any error that occurred
func (s *BundleService) Upload(clusterID string, data []byte) (int64, error) {
	// Validate bundle
	result := bundle.Validate(data)
	if !result.Valid {
		return 0, result.Error
	}

	s.logger.Info("bundle validation passed",
		zap.String("cluster_id", clusterID),
		zap.Int64("size", result.Size),
		zap.Int("files", len(result.Files)),
	)

	// Start transaction for atomic version increment and bundle storage
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current version and increment
	var currentVersion int64
	err = tx.QueryRow(`SELECT config_version FROM clusters WHERE id = ?`, clusterID).Scan(&currentVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	newVersion := currentVersion + 1

	// Update cluster version
	_, err = tx.Exec(`UPDATE clusters SET config_version = ? WHERE id = ?`, newVersion, clusterID)
	if err != nil {
		return 0, fmt.Errorf("failed to update cluster version: %w", err)
	}

	// Insert bundle
	now := time.Now()
	_, err = tx.Exec(`
		INSERT INTO config_bundles (cluster_id, version, data, created_at)
		VALUES (?, ?, ?, ?)
	`, clusterID, newVersion, data, now)
	if err != nil {
		return 0, fmt.Errorf("failed to insert bundle: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("config bundle uploaded",
		zap.String("cluster_id", clusterID),
		zap.Int64("version", newVersion),
		zap.Int("size_bytes", len(data)),
	)

	return newVersion, nil
}

// GetCurrentVersion returns the current config version for a cluster.
//
// Parameters:
//   - clusterID: The cluster ID
//
// Returns:
//   - int64: The current version number
//   - error: Any error that occurred
func (s *BundleService) GetCurrentVersion(clusterID string) (int64, error) {
	var version int64
	err := s.db.QueryRow(`SELECT config_version FROM clusters WHERE id = ?`, clusterID).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("cluster not found: %s", clusterID)
	} else if err != nil {
		return 0, fmt.Errorf("failed to get version: %w", err)
	}

	return version, nil
}

// Download retrieves a config bundle by version.
//
// If version is 0, returns the latest bundle.
//
// Parameters:
//   - clusterID: The cluster ID
//   - version: The version to retrieve (0 for latest)
//
// Returns:
//   - []byte: The bundle data
//   - int64: The bundle version
//   - error: Any error that occurred
func (s *BundleService) Download(clusterID string, version int64) ([]byte, int64, error) {
	var data []byte
	var actualVersion int64

	var query string
	var args []interface{}

	if version == 0 {
		// Get latest version
		query = `
			SELECT version, data
			FROM config_bundles
			WHERE cluster_id = ?
			ORDER BY version DESC
			LIMIT 1
		`
		args = []interface{}{clusterID}
	} else {
		// Get specific version
		query = `
			SELECT version, data
			FROM config_bundles
			WHERE cluster_id = ? AND version = ?
		`
		args = []interface{}{clusterID, version}
	}

	err := s.db.QueryRow(query, args...).Scan(&actualVersion, &data)
	if err == sql.ErrNoRows {
		if version == 0 {
			return nil, 0, fmt.Errorf("no bundles found for cluster: %s", clusterID)
		}
		return nil, 0, fmt.Errorf("bundle version %d not found for cluster: %s", version, clusterID)
	} else if err != nil {
		return nil, 0, fmt.Errorf("failed to download bundle: %w", err)
	}

	s.logger.Debug("config bundle downloaded",
		zap.String("cluster_id", clusterID),
		zap.Int64("version", actualVersion),
		zap.Int("size_bytes", len(data)),
	)

	return data, actualVersion, nil
}

// CheckVersion checks if a client's version is current.
//
// Returns true if the client has the latest version, false otherwise.
//
// Parameters:
//   - clusterID: The cluster ID
//   - clientVersion: The client's current version
//
// Returns:
//   - bool: true if client is up-to-date
//   - int64: The current version number
//   - error: Any error that occurred
func (s *BundleService) CheckVersion(clusterID string, clientVersion int64) (bool, int64, error) {
	currentVersion, err := s.GetCurrentVersion(clusterID)
	if err != nil {
		return false, 0, err
	}

	return clientVersion == currentVersion, currentVersion, nil
}
