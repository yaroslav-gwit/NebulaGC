-- +goose Up
-- Create clusters table for storing Nebula cluster configurations.
-- Each cluster has its own CA certificate, versioned config bundles,
-- and optional lighthouse functionality provided by the control plane.
CREATE TABLE clusters (
    id TEXT PRIMARY KEY,                     -- UUID v4
    tenant_id TEXT NOT NULL,                 -- Foreign key to tenants.id
    name TEXT NOT NULL,                      -- Human-readable cluster name
    cluster_token_hash TEXT NOT NULL,        -- HMAC-SHA256 hash of shared cluster secret
    provide_lighthouse INTEGER NOT NULL DEFAULT 0 CHECK(provide_lighthouse IN (0,1)), -- Boolean: control plane acts as lighthouse
    lighthouse_port INTEGER DEFAULT 4242,    -- UDP port for lighthouse traffic
    config_version INTEGER NOT NULL DEFAULT 1, -- Current config version (incremented on changes)
    pki_ca_cert TEXT,                        -- PEM-encoded CA certificate
    pki_ca_key TEXT,                         -- PEM-encoded CA private key (encrypted)
    pki_crl TEXT,                            -- PEM-encoded Certificate Revocation List
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
    UNIQUE(tenant_id, name)                  -- Cluster names must be unique within tenant
);

-- Index for tenant-based queries
CREATE INDEX idx_clusters_tenant ON clusters(tenant_id);

-- Index for version-based queries
CREATE INDEX idx_clusters_version ON clusters(config_version);

-- +goose Down
DROP INDEX IF EXISTS idx_clusters_version;
DROP INDEX IF EXISTS idx_clusters_tenant;
DROP TABLE IF EXISTS clusters;
