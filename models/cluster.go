package models

import "time"

// Cluster represents a logical Nebula overlay network environment.
// A cluster belongs to exactly one tenant and contains multiple nodes.
// Each cluster has its own CA certificate, configuration versioning, and optional
// lighthouse functionality provided by the control plane.
type Cluster struct {
	// ID is the unique identifier for this cluster (UUID v4 format)
	ID string `json:"id" db:"id"`

	// TenantID is the UUID of the owning tenant
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// Name is the human-readable cluster name (e.g., "prod-eu-west", "staging")
	// Maximum length: 255 characters
	// Must be unique within the tenant
	Name string `json:"name" db:"name"`

	// ClusterTokenHash is the HMAC-SHA256 hash of the cluster's shared secret
	// The actual token is returned only when the cluster is created
	// This token is shared by all nodes in the cluster for authentication
	ClusterTokenHash string `json:"-" db:"cluster_token_hash"`

	// ProvideLighthouse indicates whether the control plane acts as a lighthouse
	// for this cluster. If true, all control plane instances (master + replicas)
	// will spawn Nebula lighthouse processes for N-way redundancy.
	ProvideLighthouse bool `json:"provide_lighthouse" db:"provide_lighthouse"`

	// LighthousePort is the UDP port used for lighthouse traffic
	// Default: 4242
	// Only used if ProvideLighthouse is true
	LighthousePort int `json:"lighthouse_port,omitempty" db:"lighthouse_port"`

	// ConfigVersion is the current configuration version for this cluster
	// Incremented whenever PKI changes, node topology changes, or routes are updated
	// Nodes compare this against their local version to detect updates
	ConfigVersion int64 `json:"config_version" db:"config_version"`

	// PKICACert is the PEM-encoded CA certificate for this cluster
	// Stored in the database so any control plane instance can issue certificates
	PKICACert string `json:"pki_ca_cert,omitempty" db:"pki_ca_cert"`

	// PKICAKey is the PEM-encoded CA private key (encrypted at rest)
	// Stored in the database so any control plane instance can sign certificates
	// Never returned in API responses
	PKICAKey string `json:"-" db:"pki_ca_key"`

	// PKICRL is the PEM-encoded Certificate Revocation List
	// Updated when node certificates are revoked
	PKICRL string `json:"pki_crl,omitempty" db:"pki_crl"`

	// CreatedAt is the timestamp when this cluster was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ClusterCreateRequest represents the request body for creating a new cluster.
type ClusterCreateRequest struct {
	// TenantID is the UUID of the owning tenant (required)
	TenantID string `json:"tenant_id" binding:"required,uuid4"`

	// Name is the desired cluster name (required)
	// Must be 1-255 characters
	Name string `json:"name" binding:"required,min=1,max=255"`

	// ProvideLighthouse indicates if control plane should act as lighthouse
	// Default: false
	ProvideLighthouse bool `json:"provide_lighthouse"`

	// LighthousePort is the UDP port for lighthouse traffic
	// Default: 4242
	// Only used if ProvideLighthouse is true
	LighthousePort int `json:"lighthouse_port,omitempty"`
}

// ClusterCreateResponse represents the response after creating a cluster.
type ClusterCreateResponse struct {
	// Cluster is the created cluster (without sensitive fields)
	Cluster Cluster `json:"cluster"`

	// ClusterToken is the shared secret for this cluster
	// This is the only time this token is returned
	// All nodes in this cluster must use this token for authentication
	// Minimum 41 characters
	ClusterToken string `json:"cluster_token"`
}

// ClusterListResponse represents the response for listing clusters.
type ClusterListResponse struct {
	// Clusters is the list of clusters for the specified tenant
	Clusters []Cluster `json:"clusters"`

	// Total is the total number of clusters
	Total int `json:"total"`
}

// ClusterState tracks the running configuration version for each control plane instance.
// This enables each instance to independently manage its lighthouse processes.
type ClusterState struct {
	// ClusterID is the UUID of the cluster
	ClusterID string `json:"cluster_id" db:"cluster_id"`

	// InstanceID is the unique identifier for this control plane instance
	// Set via NEBULAGC_INSTANCE_ID environment variable or auto-generated
	InstanceID string `json:"instance_id" db:"instance_id"`

	// RunningConfigVersion is the config version currently running on this instance
	// Compared against Cluster.ConfigVersion to detect when restarts are needed
	RunningConfigVersion int64 `json:"running_config_version" db:"running_config_version"`

	// UpdatedAt is the timestamp when this state was last updated
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
