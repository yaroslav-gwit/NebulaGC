// Package service provides business logic services for the NebulaGC control plane.
//
// This package implements the service layer that sits between HTTP handlers
// and the database layer, containing business logic for replicas, nodes,
// clusters, and other domain entities.
package service

import (
	"database/sql"
	"fmt"
	"time"

	"go.uber.org/zap"
	"nebulagc.io/server/internal/ha"
)

// ReplicaService provides operations for managing control plane replicas.
type ReplicaService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewReplicaService creates a new replica service.
//
// Parameters:
//   - db: Database connection
//   - logger: Zap logger for structured logging
//
// Returns:
//   - Configured ReplicaService
func NewReplicaService(db *sql.DB, logger *zap.Logger) *ReplicaService {
	return &ReplicaService{
		db:     db,
		logger: logger,
	}
}

// Register registers this control plane instance in the replicas table.
//
// If the instance already exists (restart scenario), updates the URL and heartbeat.
// If not exists, inserts a new replica record.
//
// Parameters:
//   - instanceID: This instance's UUID
//   - address: This instance's public address
//   - mode: The runtime mode (master or replica)
//
// Returns:
//   - error: Any error that occurred during registration
func (s *ReplicaService) Register(instanceID, address string, mode ha.Mode) error {
	if !ha.ValidateMode(mode) {
		return fmt.Errorf("invalid mode %q: must be master or replica", mode)
	}

	// Check if replica already exists
	var existingID string
	checkQuery := `SELECT id FROM replicas WHERE id = ?`
	err := s.db.QueryRow(checkQuery, instanceID).Scan(&existingID)

	now := time.Now()

	if err == sql.ErrNoRows {
		// New replica - insert
		insertQuery := `
			INSERT INTO replicas (id, address, role, last_seen_at, created_at)
			VALUES (?, ?, ?, ?, ?)
		`
		_, err = s.db.Exec(insertQuery, instanceID, address, string(mode), now, now)
		if err != nil {
			return fmt.Errorf("failed to register replica: %w", err)
		}

		s.logger.Info("registered new replica",
			zap.String("instance_id", instanceID),
			zap.String("address", address),
			zap.String("role", string(mode)),
		)
	} else if err != nil {
		return fmt.Errorf("failed to check replica existence: %w", err)
	} else {
		// Existing replica - update (restart scenario)
		updateQuery := `
			UPDATE replicas
			SET address = ?, role = ?, last_seen_at = ?
			WHERE id = ?
		`
		_, err = s.db.Exec(updateQuery, address, string(mode), now, instanceID)
		if err != nil {
			return fmt.Errorf("failed to update replica: %w", err)
		}

		s.logger.Info("updated existing replica",
			zap.String("instance_id", instanceID),
			zap.String("address", address),
			zap.String("role", string(mode)),
		)
	}

	return nil
}

// SendHeartbeat updates the last_heartbeat timestamp for this instance.
//
// This should be called periodically (e.g., every 10 seconds) to indicate
// that this instance is still alive and healthy.
//
// Parameters:
//   - instanceID: This instance's UUID
//
// Returns:
//   - error: Any error that occurred during heartbeat
func (s *ReplicaService) SendHeartbeat(instanceID string) error {
	query := `
		UPDATE replicas
		SET last_seen_at = ?
		WHERE id = ?
	`

	result, err := s.db.Exec(query, time.Now(), instanceID)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check heartbeat result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("replica not found for heartbeat: %s", instanceID)
	}

	s.logger.Debug("sent heartbeat", zap.String("instance_id", instanceID))
	return nil
}

// GetMaster determines the current master replica.
//
// The master is the oldest healthy replica (by created_at) with a recent heartbeat.
// This provides deterministic, consistent master selection across all replicas.
//
// Parameters:
//   - threshold: How long before a replica is considered stale
//   - currentInstanceID: This instance's UUID (to determine if we are master)
//
// Returns:
//   - *ha.MasterInfo: Information about the current master
//   - error: Any error that occurred during master determination
func (s *ReplicaService) GetMaster(threshold time.Duration, currentInstanceID string) (*ha.MasterInfo, error) {
	cutoff := time.Now().Add(-threshold)

	query := `
		SELECT id, address, role
		FROM replicas
		WHERE last_seen_at > ?
		ORDER BY created_at ASC
		LIMIT 1
	`

	var masterID, masterAddress string
	var masterRole string
	err := s.db.QueryRow(query, cutoff).Scan(&masterID, &masterAddress, &masterRole)

	if err == sql.ErrNoRows {
		// No healthy replicas found - this shouldn't happen but we'll
		// assume current instance is master to allow operations to continue
		s.logger.Warn("no healthy replicas found, assuming self as master",
			zap.String("instance_id", currentInstanceID),
		)
		return &ha.MasterInfo{
			InstanceID: currentInstanceID,
			Address:    "",
			IsSelf:     true,
		}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to query master: %w", err)
	}

	if !ha.ValidateMode(ha.Mode(masterRole)) {
		s.logger.Warn("master record has invalid role, treating as replica",
			zap.String("instance_id", masterID),
			zap.String("role", masterRole),
		)
	}

	return &ha.MasterInfo{
		InstanceID: masterID,
		Address:    masterAddress,
		IsSelf:     masterID == currentInstanceID,
	}, nil
}

// ListReplicas returns all replicas with recent heartbeats.
//
// Parameters:
//   - threshold: How long before a replica is considered stale
//   - currentInstanceID: This instance's UUID (to mark master)
//
// Returns:
//   - []*ha.ReplicaInfo: List of healthy replicas
//   - error: Any error that occurred during query
func (s *ReplicaService) ListReplicas(threshold time.Duration, currentInstanceID string) ([]*ha.ReplicaInfo, error) {
	cutoff := time.Now().Add(-threshold)

	query := `
		SELECT id, address, role, last_seen_at, created_at
		FROM replicas
		WHERE last_seen_at > ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.Query(query, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to list replicas: %w", err)
	}
	defer rows.Close()

	var replicas []*ha.ReplicaInfo
	first := true

	for rows.Next() {
		var r ha.ReplicaInfo
		var role string
		err := rows.Scan(&r.InstanceID, &r.Address, &role, &r.LastHeartbeat, &r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan replica: %w", err)
		}

		if ha.ValidateMode(ha.Mode(role)) {
			r.Role = ha.Mode(role)
		} else {
			r.Role = ha.ModeReplica
		}

		// First replica in list is the master (ordered by created_at ASC)
		r.IsMaster = first
		first = false

		replicas = append(replicas, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating replicas: %w", err)
	}

	return replicas, nil
}

// PruneStale removes replicas with very old heartbeats.
//
// This prevents the replicas table from growing indefinitely with dead instances.
// Uses a threshold multiplier (typically 2x) to avoid pruning replicas that might
// still be in the process of restarting.
//
// Parameters:
//   - threshold: Base heartbeat threshold
//   - multiplier: Multiplier for pruning (e.g., 2 = prune after 2x threshold)
//
// Returns:
//   - int: Number of replicas pruned
//   - error: Any error that occurred during pruning
func (s *ReplicaService) PruneStale(threshold time.Duration, multiplier int) (int, error) {
	cutoff := time.Now().Add(-threshold * time.Duration(multiplier))

	query := `
		DELETE FROM replicas
		WHERE last_seen_at < ?
	`

	result, err := s.db.Exec(query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to prune stale replicas: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check prune result: %w", err)
	}

	if rows > 0 {
		s.logger.Info("pruned stale replicas",
			zap.Int64("count", rows),
			zap.Duration("threshold", threshold*time.Duration(multiplier)),
		)
	}

	return int(rows), nil
}

// Unregister removes this instance from the replicas table.
//
// This should be called during graceful shutdown to immediately remove
// this instance from the replica set rather than waiting for pruning.
//
// Parameters:
//   - instanceID: This instance's UUID
//
// Returns:
//   - error: Any error that occurred during unregistration
func (s *ReplicaService) Unregister(instanceID string) error {
	query := `DELETE FROM replicas WHERE id = ?`

	result, err := s.db.Exec(query, instanceID)
	if err != nil {
		return fmt.Errorf("failed to unregister replica: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check unregister result: %w", err)
	}

	if rows > 0 {
		s.logger.Info("unregistered replica", zap.String("instance_id", instanceID))
	}

	return nil
}

// ValidateSingleMaster ensures there is at most one master entry.
//
// This should be called on master startup to avoid split-brain scenarios.
//
// Returns:
//   - error: If multiple masters are registered or the query fails
func (s *ReplicaService) ValidateSingleMaster() error {
	const query = `SELECT COUNT(*) FROM replicas WHERE role = 'master'`

	var count int
	if err := s.db.QueryRow(query).Scan(&count); err != nil {
		return fmt.Errorf("failed to count masters: %w", err)
	}

	if count > 1 {
		return fmt.Errorf("detected %d masters in registry", count)
	}

	return nil
}
