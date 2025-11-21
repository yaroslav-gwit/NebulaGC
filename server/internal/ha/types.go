// Package ha provides high availability functionality for the NebulaGC control plane.
//
// This package implements replica registration, heartbeat mechanisms, master election,
// and automatic failover for N-way control plane replication.
package ha

import "time"

const (
	// DefaultHeartbeatInterval is how often replicas send heartbeats.
	// Default: 10 seconds
	DefaultHeartbeatInterval = 10 * time.Second

	// DefaultHeartbeatThreshold is how long before a replica is considered stale.
	// Default: 30 seconds (3x heartbeat interval)
	DefaultHeartbeatThreshold = 30 * time.Second

	// DefaultPruneInterval is how often to prune stale replicas.
	// Default: 5 minutes
	DefaultPruneInterval = 5 * time.Minute

	// PruneThresholdMultiplier is applied to heartbeat threshold for pruning.
	// Replicas older than (threshold * multiplier) are pruned.
	// Default: 2x (60 seconds with default threshold)
	PruneThresholdMultiplier = 2
)

// Config holds configuration for the HA manager.
type Config struct {
	// InstanceID is this control plane instance's UUID.
	InstanceID string

	// URL is the public URL of this instance (e.g., "https://cp1.example.com:8080").
	URL string

	// HeartbeatInterval is how often to send heartbeats.
	HeartbeatInterval time.Duration

	// HeartbeatThreshold is how long before a replica is considered stale.
	HeartbeatThreshold time.Duration

	// PruneInterval is how often to prune stale replicas.
	PruneInterval time.Duration

	// EnablePruning enables automatic pruning of stale replicas.
	// Set to false if you want manual pruning control.
	EnablePruning bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig(instanceID, url string) *Config {
	return &Config{
		InstanceID:         instanceID,
		URL:                url,
		HeartbeatInterval:  DefaultHeartbeatInterval,
		HeartbeatThreshold: DefaultHeartbeatThreshold,
		PruneInterval:      DefaultPruneInterval,
		EnablePruning:      true,
	}
}

// ReplicaInfo holds information about a control plane replica.
type ReplicaInfo struct {
	// InstanceID is the replica's UUID.
	InstanceID string

	// URL is the replica's public URL.
	URL string

	// IsMaster indicates if this replica is currently the master.
	IsMaster bool

	// LastHeartbeat is when the replica last sent a heartbeat.
	LastHeartbeat time.Time

	// CreatedAt is when the replica was first registered.
	CreatedAt time.Time
}

// MasterInfo holds information about the current master replica.
type MasterInfo struct {
	// InstanceID is the master's UUID.
	InstanceID string

	// URL is the master's public URL.
	URL string

	// IsSelf indicates if this instance is the master.
	IsSelf bool
}
