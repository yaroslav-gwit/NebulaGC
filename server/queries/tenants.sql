-- Tenant queries
-- These queries handle CRUD operations for tenant records.

-- name: GetTenant :one
-- GetTenant retrieves a single tenant by ID.
-- Returns sql.ErrNoRows if the tenant doesn't exist.
SELECT * FROM tenants
WHERE id = ?
LIMIT 1;

-- name: ListTenants :many
-- ListTenants returns all tenants in the system,
-- ordered by creation date (newest first).
SELECT * FROM tenants
ORDER BY created_at DESC;

-- name: CreateTenant :one
-- CreateTenant inserts a new tenant record and returns the created row.
INSERT INTO tenants (
    id,
    name,
    created_at
) VALUES (
    ?,
    ?,
    CURRENT_TIMESTAMP
)
RETURNING *;

-- name: DeleteTenant :exec
-- DeleteTenant removes a tenant and all associated clusters/nodes (CASCADE).
DELETE FROM tenants
WHERE id = ?;

-- name: CountTenants :one
-- CountTenants returns the total number of tenants.
SELECT COUNT(*) FROM tenants;
