-- Replica queries
-- These queries manage the control plane instance registry for HA.

-- name: GetReplica :one
-- GetReplica retrieves a single replica by ID.
SELECT * FROM replicas
WHERE id = ?
LIMIT 1;

-- name: GetReplicaByAddress :one
-- GetReplicaByAddress retrieves a replica by its address.
SELECT * FROM replicas
WHERE address = ?
LIMIT 1;

-- name: ListReplicas :many
-- ListReplicas returns all registered control plane instances.
SELECT * FROM replicas
ORDER BY created_at ASC;

-- name: ListHealthyReplicas :many
-- ListHealthyReplicas returns replicas with recent heartbeats (< 5 minutes).
SELECT * FROM replicas
WHERE last_seen_at IS NOT NULL
  AND last_seen_at > datetime('now', '-5 minutes')
ORDER BY created_at ASC;

-- name: GetMaster :one
-- GetMaster retrieves the master instance.
-- Should only return one row; multiple masters indicate a configuration error.
SELECT * FROM replicas
WHERE role = 'master'
LIMIT 1;

-- name: UpsertReplica :exec
-- UpsertReplica inserts or updates a replica registration.
-- Used during startup to register the instance.
INSERT INTO replicas (
    id,
    address,
    role,
    created_at,
    last_seen_at
) VALUES (
    ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT(id) DO UPDATE SET
    address = excluded.address,
    role = excluded.role,
    last_seen_at = CURRENT_TIMESTAMP;

-- name: UpdateReplicaHeartbeat :exec
-- UpdateReplicaHeartbeat updates the last_seen_at timestamp.
-- Called every 30 seconds by the heartbeat goroutine.
UPDATE replicas
SET last_seen_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteReplica :exec
-- DeleteReplica removes a replica from the registry.
DELETE FROM replicas
WHERE id = ?;

-- name: PruneStaleReplicas :exec
-- PruneStaleReplicas removes replicas with no heartbeat for > 5 minutes.
-- Run periodically by the master.
DELETE FROM replicas
WHERE last_seen_at IS NOT NULL
  AND last_seen_at < datetime('now', '-5 minutes');

-- name: CountMasters :one
-- CountMasters returns the number of instances with role='master'.
-- Should always be 0 or 1; > 1 indicates a split-brain condition.
SELECT COUNT(*) FROM replicas
WHERE role = 'master';
