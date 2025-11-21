-- Config bundle queries
-- These queries handle versioned configuration archive storage and retrieval.

-- name: GetBundle :one
-- GetBundle retrieves a specific bundle version for a cluster.
SELECT * FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ? AND version = ?
LIMIT 1;

-- name: GetLatestBundle :one
-- GetLatestBundle retrieves the most recent bundle for a cluster.
SELECT * FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ?
ORDER BY version DESC
LIMIT 1;

-- name: GetLatestBundleVersion :one
-- GetLatestBundleVersion returns just the version number of the latest bundle.
-- Used for quick version checks without fetching bundle data.
SELECT version FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ?
ORDER BY version DESC
LIMIT 1;

-- name: ListBundles :many
-- ListBundles returns all bundle metadata (without data) for a cluster.
SELECT version, tenant_id, cluster_id, created_by, created_at
FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ?
ORDER BY version DESC
LIMIT ? OFFSET ?;

-- name: CreateBundle :one
-- CreateBundle inserts a new config bundle with the next version number.
-- Version is calculated as MAX(version) + 1 for the cluster.
INSERT INTO config_bundles (
    version,
    tenant_id,
    cluster_id,
    data,
    created_by,
    created_at
) VALUES (
    (SELECT COALESCE(MAX(version), 0) + 1 FROM config_bundles WHERE tenant_id = ? AND cluster_id = ?),
    ?, ?, ?, ?, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: DeleteBundle :exec
-- DeleteBundle removes a specific bundle version.
DELETE FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ? AND version = ?;

-- name: DeleteOldBundles :exec
-- DeleteOldBundles removes bundles older than the specified version.
-- Useful for cleanup while keeping recent history.
DELETE FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ? AND version < ?;

-- name: CountBundles :one
-- CountBundles returns the total number of bundles for a cluster.
SELECT COUNT(*) FROM config_bundles
WHERE tenant_id = ? AND cluster_id = ?;
