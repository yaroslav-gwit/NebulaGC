-- Cluster queries
-- These queries handle CRUD operations for cluster records and config versioning.

-- name: GetCluster :one
-- GetCluster retrieves a single cluster by ID within a tenant scope.
-- Returns sql.ErrNoRows if the cluster doesn't exist.
SELECT * FROM clusters
WHERE id = ? AND tenant_id = ?
LIMIT 1;

-- name: GetClusterByID :one
-- GetClusterByID retrieves a cluster by ID without tenant scoping (for internal use).
SELECT * FROM clusters
WHERE id = ?
LIMIT 1;

-- name: ListClusters :many
-- ListClusters returns all clusters for a given tenant,
-- ordered by creation date (newest first).
SELECT * FROM clusters
WHERE tenant_id = ?
ORDER BY created_at DESC;

-- name: ListAllClusters :many
-- ListAllClusters returns all clusters across all tenants (for admin operations).
SELECT * FROM clusters
ORDER BY created_at DESC;

-- name: CreateCluster :one
-- CreateCluster inserts a new cluster record and returns the created row.
INSERT INTO clusters (
    id,
    tenant_id,
    name,
    cluster_token_hash,
    provide_lighthouse,
    lighthouse_port,
    config_version,
    pki_ca_cert,
    pki_ca_key,
    pki_crl,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: UpdateClusterVersion :exec
-- UpdateClusterVersion increments the config version for a cluster.
-- This triggers lighthouse restarts on all control plane instances.
UPDATE clusters
SET config_version = config_version + 1
WHERE id = ? AND tenant_id = ?;

-- name: UpdateClusterTokenHash :exec
-- UpdateClusterTokenHash updates the cluster token hash (for token rotation).
UPDATE clusters
SET cluster_token_hash = ?
WHERE id = ? AND tenant_id = ?;

-- name: UpdateClusterPKI :exec
-- UpdateClusterPKI updates the PKI fields (CA cert, key, CRL).
UPDATE clusters
SET pki_ca_cert = ?,
    pki_ca_key = ?,
    pki_crl = ?,
    config_version = config_version + 1
WHERE id = ? AND tenant_id = ?;

-- name: DeleteCluster :exec
-- DeleteCluster removes a cluster and all associated nodes/bundles (CASCADE).
DELETE FROM clusters
WHERE id = ? AND tenant_id = ?;

-- name: CountClusters :one
-- CountClusters returns the total number of clusters for a tenant.
SELECT COUNT(*) FROM clusters
WHERE tenant_id = ?;

-- name: GetClusterVersion :one
-- GetClusterVersion returns just the config version for a cluster.
-- Used for quick version checks without fetching full cluster data.
SELECT config_version FROM clusters
WHERE id = ? AND tenant_id = ?
LIMIT 1;
