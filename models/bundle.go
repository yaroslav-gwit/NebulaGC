package models

import "time"

// ConfigBundle represents a versioned configuration archive for a Nebula cluster.
// Bundles are tar.gz files containing all files needed to run Nebula:
//   - config.yml: Nebula configuration
//   - ca.crt: Certificate Authority certificate
//   - crl.pem: Certificate Revocation List
//   - host.crt: Host certificate
//   - host.key: Host private key
//   - lighthouses.json: Optional lighthouse list helper
//
// Each bundle is associated with a specific version number that increments
// when the cluster configuration changes.
type ConfigBundle struct {
	// Version is the configuration version number for this bundle
	// Starts at 1 and increments with each upload or topology change
	Version int64 `json:"version" db:"version"`

	// TenantID is the UUID of the owning tenant
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// ClusterID is the UUID of the cluster this bundle belongs to
	ClusterID string `json:"cluster_id" db:"cluster_id"`

	// Data is the raw tar.gz bundle data
	// Maximum size: 10 MiB (10,485,760 bytes)
	Data []byte `json:"-" db:"data"`

	// CreatedBy is the UUID of the node that uploaded this bundle
	// May be null if the node was deleted after upload
	CreatedBy *string `json:"created_by,omitempty" db:"created_by"`

	// CreatedAt is the timestamp when this bundle was uploaded
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// BundleVersionResponse represents the response for checking the latest bundle version.
type BundleVersionResponse struct {
	// LatestVersion is the most recent configuration version available
	LatestVersion int64 `json:"latest_version"`
}

// BundleUploadResponse represents the response after uploading a config bundle.
type BundleUploadResponse struct {
	// Version is the version number assigned to this bundle
	// Automatically incremented from the previous version
	Version int64 `json:"version"`

	// UploadedAt is the timestamp when the bundle was uploaded
	UploadedAt time.Time `json:"uploaded_at"`
}

// BundleValidationError represents an error during bundle validation.
type BundleValidationError struct {
	// Message is the human-readable error message
	Message string `json:"message"`

	// Field indicates which part of the bundle failed validation
	// Examples: "size", "format", "config.yml", "ca.crt"
	Field string `json:"field,omitempty"`
}

// Error implements the error interface for BundleValidationError.
func (e *BundleValidationError) Error() string {
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}
