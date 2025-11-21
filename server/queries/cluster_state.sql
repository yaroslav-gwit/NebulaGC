-- Cluster state queries
-- These queries manage per-instance lighthouse version tracking.

-- name: GetClusterState :one
-- GetClusterState retrieves the running config version for a specific instance.
SELECT * FROM cluster_state
WHERE cluster_id = ? AND instance_id = ?
LIMIT 1;

-- name: ListClusterStates :many
-- ListClusterStates returns all instance states for a cluster.
SELECT * FROM cluster_state
WHERE cluster_id = ?
ORDER BY updated_at DESC;

-- name: UpsertClusterState :exec
-- UpsertClusterState inserts or updates the running config version for an instance.
INSERT INTO cluster_state (
    cluster_id,
    instance_id,
    running_config_version,
    updated_at
) VALUES (
    ?, ?, ?, CURRENT_TIMESTAMP
)
ON CONFLICT(cluster_id, instance_id) DO UPDATE SET
    running_config_version = excluded.running_config_version,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetStaleClusters :many
-- GetStaleClusters finds clusters where running_config_version < config_version
-- for a specific instance, indicating lighthouse restart is needed.
SELECT c.id, c.config_version, COALESCE(cs.running_config_version, 0) as running_config_version
FROM clusters c
LEFT JOIN cluster_state cs ON c.id = cs.cluster_id AND cs.instance_id = ?
WHERE c.provide_lighthouse = 1
  AND (cs.running_config_version IS NULL OR cs.running_config_version < c.config_version);

-- name: DeleteClusterState :exec
-- DeleteClusterState removes state tracking for a specific instance.
DELETE FROM cluster_state
WHERE cluster_id = ? AND instance_id = ?;
