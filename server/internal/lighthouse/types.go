// Package lighthouse provides automated Nebula lighthouse process management.
//
// This package implements process lifecycle management, configuration generation,
// and automatic restarts for Nebula lighthouse instances running on control plane servers.
package lighthouse

import "time"

// Config holds configuration for the lighthouse manager.
type Config struct {
	// InstanceID is this control plane instance's UUID.
	InstanceID string

	// BasePath is the base directory for lighthouse configs and PKI.
	// Default: /var/lib/nebulagc/lighthouse
	BasePath string

	// NebulaBinary is the path to the nebula binary.
	// Default: /usr/local/bin/nebula
	NebulaBinary string

	// CheckInterval is how often to check for config version updates.
	// Default: 5 seconds
	CheckInterval time.Duration

	// Enabled determines if lighthouse management is enabled.
	// Default: true
	Enabled bool
}

// DefaultConfig returns a Config with default values.
func DefaultConfig(instanceID string) *Config {
	return &Config{
		InstanceID:    instanceID,
		BasePath:      "/var/lib/nebulagc/lighthouse",
		NebulaBinary:  "/usr/local/bin/nebula",
		CheckInterval: 5 * time.Second,
		Enabled:       true,
	}
}

// ClusterConfig holds the configuration data needed to run a lighthouse.
type ClusterConfig struct {
	// ClusterID is the cluster's UUID.
	ClusterID string

	// ClusterName is the human-readable cluster name.
	ClusterName string

	// CACert is the CA certificate in PEM format.
	CACert string

	// CRL is the certificate revocation list in PEM format.
	CRL string

	// HostCert is the lighthouse's host certificate in PEM format.
	HostCert string

	// HostKey is the lighthouse's private key in PEM format.
	HostKey string

	// LighthousePort is the UDP port for Nebula.
	LighthousePort int

	// ConfigVersion is the current config version from the database.
	ConfigVersion int64

	// Replicas is the list of all control plane instances (for static host map).
	Replicas []ReplicaInfo
}

// ReplicaInfo holds information about a control plane replica for config generation.
type ReplicaInfo struct {
	// InstanceID is the replica's UUID.
	InstanceID string

	// Address is the replica's public URL.
	Address string
}

// ProcessInfo tracks a running Nebula process.
type ProcessInfo struct {
	// ClusterID is the cluster UUID.
	ClusterID string

	// PID is the process ID.
	PID int

	// ConfigVersion is the version this process is running.
	ConfigVersion int64

	// StartedAt is when the process was started.
	StartedAt time.Time
}
