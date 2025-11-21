-- +goose Up
-- Create config_bundles table for storing versioned configuration archives.
-- Each bundle is a tar.gz file containing Nebula config, certificates, and CRL.
-- Bundles are immutable - new uploads create new versions.
CREATE TABLE config_bundles (
    version INTEGER NOT NULL,                -- Version number (incremented per cluster)
    tenant_id TEXT NOT NULL,                 -- Foreign key to tenants.id
    cluster_id TEXT NOT NULL,                -- Foreign key to clusters.id
    data BLOB NOT NULL,                      -- Tar.gz archive data (max 10 MiB)
    created_by TEXT,                         -- Foreign key to nodes.id (nullable - node may be deleted)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, cluster_id, version),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
    FOREIGN KEY (created_by) REFERENCES nodes(id) ON DELETE SET NULL
);

-- Index for cluster-based queries
CREATE INDEX idx_config_bundles_cluster ON config_bundles(tenant_id, cluster_id);

-- Index for version-ordered queries (most recent first)
CREATE INDEX idx_config_bundles_cluster_version ON config_bundles(tenant_id, cluster_id, version DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_config_bundles_cluster_version;
DROP INDEX IF EXISTS idx_config_bundles_cluster;
DROP TABLE IF EXISTS config_bundles;
