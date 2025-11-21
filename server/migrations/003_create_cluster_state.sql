-- +goose Up
-- Create cluster_state table for tracking running config version per control plane instance.
-- Each control plane instance (master or replica) independently tracks which version
-- it's currently running for lighthouse management.
CREATE TABLE cluster_state (
    cluster_id TEXT NOT NULL,                -- Foreign key to clusters.id
    instance_id TEXT NOT NULL,               -- Control plane instance UUID
    running_config_version INTEGER NOT NULL DEFAULT 0, -- Version currently running on this instance
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (cluster_id, instance_id),
    FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);

-- Index for checking version staleness
CREATE INDEX idx_cluster_state_version ON cluster_state(cluster_id, running_config_version);

-- +goose Down
DROP INDEX IF EXISTS idx_cluster_state_version;
DROP TABLE IF EXISTS cluster_state;
