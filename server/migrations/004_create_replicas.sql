-- +goose Up
-- Create replicas table for control plane instance registry.
-- Tracks all control plane instances (master + replicas) for client discovery
-- and health monitoring.
CREATE TABLE replicas (
    id TEXT PRIMARY KEY,                     -- Control plane instance UUID
    address TEXT NOT NULL UNIQUE,            -- Full URL: https://control1.example.com
    role TEXT NOT NULL CHECK(role IN ('master','replica')), -- Instance role
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME                    -- Last heartbeat timestamp
);

-- Index for filtering by role
CREATE INDEX idx_replicas_role ON replicas(role);

-- Index for health check queries
CREATE INDEX idx_replicas_last_seen ON replicas(last_seen_at);

-- +goose Down
DROP INDEX IF EXISTS idx_replicas_last_seen;
DROP INDEX IF EXISTS idx_replicas_role;
DROP TABLE IF EXISTS replicas;
