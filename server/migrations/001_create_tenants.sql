-- +goose Up
-- Create tenants table for storing organization records.
-- Each tenant can own multiple clusters.
CREATE TABLE tenants (
    id TEXT PRIMARY KEY,                     -- UUID v4
    name TEXT NOT NULL,                      -- Organization name (max 255 chars)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for searching by name
CREATE INDEX idx_tenants_name ON tenants(name);

-- +goose Down
DROP INDEX IF EXISTS idx_tenants_name;
DROP TABLE IF EXISTS tenants;
