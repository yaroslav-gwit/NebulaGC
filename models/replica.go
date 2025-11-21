package models

import "time"

// Replica represents a control plane instance in the HA cluster.
// The replicas table tracks all control plane servers (master + replicas)
// to enable client discovery and monitoring.
type Replica struct {
	// ID is the unique identifier for this control plane instance
	// Set via NEBULAGC_INSTANCE_ID environment variable or auto-generated UUID
	ID string `json:"id" db:"id"`

	// Address is the full URL for this control plane instance
	// Examples: "https://control1.example.com", "https://10.0.1.5:8080"
	// Must include protocol (http/https) and port if non-standard
	Address string `json:"address" db:"address"`

	// Role indicates whether this instance is the master or a replica
	// Valid values: "master", "replica"
	// Only one instance should have role="master" at any time
	Role string `json:"role" db:"role"`

	// CreatedAt is the timestamp when this instance was first registered
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// LastSeenAt is the timestamp of the last heartbeat from this instance
	// Updated every 30 seconds by the heartbeat goroutine
	// Stale entries (no heartbeat for >5 minutes) are pruned by the master
	LastSeenAt *time.Time `json:"last_seen_at,omitempty" db:"last_seen_at"`
}

// ReplicaInfo represents replica information in API responses.
type ReplicaInfo struct {
	// Address is the full URL for this control plane instance
	Address string `json:"address"`

	// Role indicates whether this instance is the master or a replica
	Role string `json:"role"`

	// Healthy indicates whether this instance is responding to health checks
	// Based on LastSeenAt timestamp (true if < 5 minutes old)
	Healthy bool `json:"healthy,omitempty"`
}

// ReplicaListResponse represents the response for listing replicas.
type ReplicaListResponse struct {
	// Instances is the list of all registered control plane instances
	Instances []ReplicaInfo `json:"instances"`

	// Total is the total number of instances
	Total int `json:"total"`
}

// CheckMasterResponse represents the response for checking master status.
type CheckMasterResponse struct {
	// Master indicates whether this instance is the master
	// true = this is the master (accepts writes)
	// false = this is a replica (read-only)
	Master bool `json:"master"`
}
