-- +goose Up
-- Create nodes table for storing Nebula node records.
-- Each node belongs to exactly one tenant and cluster, and has authentication tokens,
-- network configuration (MTU, routes), and topology role (lighthouse, relay).
CREATE TABLE nodes (
    id TEXT PRIMARY KEY,                     -- UUID v4
    tenant_id TEXT NOT NULL,                 -- Foreign key to tenants.id
    cluster_id TEXT NOT NULL,                -- Foreign key to clusters.id
    name TEXT NOT NULL,                      -- Human-readable node name
    is_admin INTEGER NOT NULL DEFAULT 0 CHECK(is_admin IN (0,1)), -- Boolean: cluster admin privileges
    token_hash TEXT NOT NULL,                -- HMAC-SHA256 hash of node authentication token
    mtu INTEGER NOT NULL DEFAULT 1300 CHECK(mtu >= 1280 AND mtu <= 9000), -- MTU size in bytes
    routes TEXT,                             -- JSON array of CIDR strings (e.g., ["10.0.0.0/8"])
    routes_updated_at DATETIME,              -- Timestamp when routes were last modified
    is_lighthouse INTEGER NOT NULL DEFAULT 0 CHECK(is_lighthouse IN (0,1)), -- Boolean: acts as lighthouse
    lighthouse_public_ip TEXT,               -- Public IP for lighthouse (required if is_lighthouse=1)
    lighthouse_port INTEGER,                 -- UDP port for lighthouse (defaults to cluster lighthouse_port)
    is_relay INTEGER NOT NULL DEFAULT 0 CHECK(is_relay IN (0,1)), -- Boolean: acts as relay
    lighthouse_relay_updated_at DATETIME,    -- Timestamp when lighthouse/relay status changed
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, cluster_id, name)      -- Node names must be unique within cluster
);

-- Index for token-based authentication lookups
CREATE INDEX idx_nodes_token_hash ON nodes(token_hash);

-- Index for cluster-based queries
CREATE INDEX idx_nodes_cluster ON nodes(cluster_id);

-- Index for tenant+cluster queries
CREATE INDEX idx_nodes_tenant_cluster ON nodes(tenant_id, cluster_id);

-- Index for lighthouse queries (partial index for performance)
CREATE INDEX idx_nodes_lighthouse ON nodes(cluster_id, is_lighthouse) WHERE is_lighthouse = 1;

-- Index for relay queries (partial index for performance)
CREATE INDEX idx_nodes_relay ON nodes(cluster_id, is_relay) WHERE is_relay = 1;

-- +goose Down
DROP INDEX IF EXISTS idx_nodes_relay;
DROP INDEX IF EXISTS idx_nodes_lighthouse;
DROP INDEX IF EXISTS idx_nodes_tenant_cluster;
DROP INDEX IF EXISTS idx_nodes_cluster;
DROP INDEX IF EXISTS idx_nodes_token_hash;
DROP TABLE IF EXISTS nodes;
