-- Node queries
-- These queries handle CRUD operations for node records, authentication, and topology.

-- name: GetNode :one
-- GetNode retrieves a single node by ID within tenant and cluster scope.
SELECT * FROM nodes
WHERE id = ? AND tenant_id = ? AND cluster_id = ?
LIMIT 1;

-- name: GetNodeByID :one
-- GetNodeByID retrieves a node by ID without scoping (for internal use).
SELECT * FROM nodes
WHERE id = ?
LIMIT 1;

-- name: GetNodeByTokenHash :one
-- GetNodeByTokenHash retrieves a node by its token hash for authentication.
SELECT * FROM nodes
WHERE token_hash = ?
LIMIT 1;

-- name: ListNodes :many
-- ListNodes returns all nodes for a cluster,
-- ordered by creation date (newest first).
SELECT * FROM nodes
WHERE tenant_id = ? AND cluster_id = ?
ORDER BY created_at DESC
LIMIT ? OFFSET ?;

-- name: ListAllNodesInCluster :many
-- ListAllNodesInCluster returns all nodes in a cluster without pagination.
SELECT * FROM nodes
WHERE cluster_id = ?
ORDER BY created_at DESC;

-- name: ListLighthouses :many
-- ListLighthouses returns all nodes designated as lighthouses in a cluster.
SELECT * FROM nodes
WHERE cluster_id = ? AND is_lighthouse = 1
ORDER BY created_at ASC;

-- name: ListRelays :many
-- ListRelays returns all nodes designated as relays in a cluster.
SELECT * FROM nodes
WHERE cluster_id = ? AND is_relay = 1
ORDER BY created_at ASC;

-- name: ListNodesWithRoutes :many
-- ListNodesWithRoutes returns all nodes that have registered routes.
SELECT * FROM nodes
WHERE cluster_id = ? AND routes IS NOT NULL AND routes != '[]'
ORDER BY created_at ASC;

-- name: CreateNode :one
-- CreateNode inserts a new node record and returns the created row.
INSERT INTO nodes (
    id,
    tenant_id,
    cluster_id,
    name,
    is_admin,
    token_hash,
    mtu,
    created_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP
)
RETURNING *;

-- name: UpdateNodeTokenHash :exec
-- UpdateNodeTokenHash updates the node's token hash (for token rotation).
UPDATE nodes
SET token_hash = ?
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: UpdateNodeMTU :exec
-- UpdateNodeMTU updates the MTU setting for a node.
UPDATE nodes
SET mtu = ?
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: UpdateNodeRoutes :exec
-- UpdateNodeRoutes updates the routes advertised by a node.
UPDATE nodes
SET routes = ?,
    routes_updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: UpdateNodeLighthouse :exec
-- UpdateNodeLighthouse updates the lighthouse configuration for a node.
UPDATE nodes
SET is_lighthouse = ?,
    lighthouse_public_ip = ?,
    lighthouse_port = ?,
    lighthouse_relay_updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: UpdateNodeRelay :exec
-- UpdateNodeRelay updates the relay status for a node.
UPDATE nodes
SET is_relay = ?,
    lighthouse_relay_updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: DeleteNode :exec
-- DeleteNode removes a node from the cluster.
DELETE FROM nodes
WHERE id = ? AND tenant_id = ? AND cluster_id = ?;

-- name: CountNodes :one
-- CountNodes returns the total number of nodes in a cluster.
SELECT COUNT(*) FROM nodes
WHERE tenant_id = ? AND cluster_id = ?;

-- name: CountAdminNodes :one
-- CountAdminNodes returns the number of admin nodes in a cluster.
SELECT COUNT(*) FROM nodes
WHERE tenant_id = ? AND cluster_id = ? AND is_admin = 1;
