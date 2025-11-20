# NebulaGC (Nebula Ground Control) — Initial Design & API / SDK Specification

This document describes the **initial implementation** of a Nebula control plane including:

- A REST API (JSON + headers-based auth) limited to per-cluster operations (node enrollment and config distribution); tenant/cluster management is intentionally *not* exposed here.
- The data model (SQLite WAL + Goose migrations + SQLc)
- A Go backend layout
- A Go client SDK (a library really, that's being imported by `nebulagc` or any other tool) to avoid messing with raw JSON
- A `nebulagc` binary (Cobra) that runs as a background daemon and polls every 5 seconds
- A version-based config distribution system

The goal is to create a small, predictable, embeddable solution usable by Hoster, OPNSense, pfSense, VyOS, and community tools.

---

# 1. Concepts & Requirements

## 1.1 Core entities

### Tenant
Represents a customer / organisation.  
Each tenant can contain one or more **clusters**.

### Cluster  
A logical Nebula environment for a tenant (e.g. `prod`, `dev`, `eu-west`, `lab`).

### Node  
A machine that:

- belongs to exactly **one tenant**
- belongs to exactly **one cluster**
- is authenticated by:
  - `node_id`
  - `node_token`
  - `tenant_id`
  - `cluster_id`
- `cluster_token`
- has a role:
  - `admin` (cluster-admin) → may create nodes in its own cluster and upload config bundles for that cluster
  - `node` → may only fetch config for its cluster

### Config Bundle  
A binary blob (typically a `tar.gz` archive) containing Nebula configuration for the cluster, including:

- `config.yml`  
- node certs  
- CA cert  
- CRL  
- lighthouse list  
- anything else needed by nodes

Each new upload increments the version number (`1, 2, 3, ...`) per `(tenant, cluster)`.

## 1.2 Operational Workflow (Anon-Friendly)
- Super admin uses the server Cobra CLI (or Unix socket) to create a tenant and its initial cluster admin node, plus the cluster folder and Nebula PKI bundle.
- Super admin hands credentials to the tenant admin.
- Tenant admin runs `nebulagc` using those admin credentials to enroll nodes over the REST API (cluster-scoped).
- Super admin can list tenants/clusters/nodes via the CLI or Unix socket to verify posture; this visibility is not exposed over REST.

---

# 2. HTTP API (Cluster-Scoped)

Base URL: `/v1`

All requests must send the following headers unless noted otherwise:

- `X-Nebula-Tenant-ID`
- `X-Nebula-Cluster-ID`
- `X-Nebula-Node-ID`
- `X-Nebula-Node-Token`
- `X-Nebula-Cluster-Token`

## Error Handling

Errors are returned in JSON format with a non-2xx status code:

```json
{
  "error": "Description of the error",
  "code": "ERROR_CODE" 
}
```

Common codes: `UNAUTHORIZED`, `NOT_FOUND`, `BAD_REQUEST`, `INTERNAL_ERROR`.

### Auth & Security Notes
- All tokens are secrets; only the `node_token` is ever returned (and only at creation). Store `token_hash` (HMAC-SHA256) in DB, never the raw token.
- Cluster-admin-only routes require the calling node to have `role=admin` in DB *and* match `(tenant_id, cluster_id)` in headers.
- Super-admin actions (tenant/cluster lifecycle, global listing) are **not** available via REST; only via server CLI or the privileged Unix socket interface.
- Rate-limit per node (simple leaky bucket; in-memory is fine for now) to avoid brute-force attempts.
- Add a minimal unauthenticated `GET /v1/healthz` that returns `200 OK` for liveness probes; no DB access needed.

### Scope & Anon-Friendly Rules
- The REST API is cluster-scoped: callers can only manage nodes and config for the cluster in their headers.
- Nodes with `role=admin` are *cluster admins*, not super admins. They can only:
  - Create nodes within their own `(tenant, cluster)`.
  - Upload/download config bundles for their cluster.
  - Rotate Nebula certificates for their cluster (by uploading a new bundle).
- Tenants and clusters cannot be listed or created over REST. Super-admin functions live in the server CLI or a privileged Unix socket API (see Section 7).

## 2.1 Create Node (Cluster Admin Only)
**POST `/v1/tenants/{tenant_id}/clusters/{cluster_id}/nodes`**

Headers: cluster-scoped auth as usual; the caller must have `role=admin` for this cluster.

Body:
```json
{
  "name": "hoster-node-01",
  "role": "node"
}
```

Response includes a `node_token` (returned only at creation).

## 2.2 Get Latest Config Version (Any Node)
**GET `/v1/config/version`**

Returns:
```json
{
  "latest_version": 4
}
```

## 2.3 Download Config Bundle (Any Node)
**GET `/v1/config/bundle?current_version=3`**

Response:
- `304 Not Modified` → nothing changed
- `200 OK` → with `X-Nebula-Config-Version: <int>` and binary body (tar.gz)

## 2.4 Upload Config Bundle (Admin Node)
**POST `/v1/config/bundle`**

Binary body (tar.gz).  
Server increments version automatically.

Returns:
```json
{ "version": 5 }
```

## 2.5 Config Bundle Contract
- Required files inside the archive:
  - `config.yml` (the Nebula config the process will read)
  - `ca.crt`, `crl.pem`, `host.crt`, `host.key` (or equivalents for your PKI model)
  - `lighthouses.json` (optional helper list; the daemon can also derive from config)
- The server stores bundles verbatim; validation is limited to size < 10 MiB and required filenames present (fail with `BAD_REQUEST`).
- Response header `X-Nebula-Config-Version` must be included on successful download for caching.

---

# 3. Database Schema (SQLite + Goose + SQLc)

Use `modernc.org/sqlite` driver.

## 3.1 tenants
```sql
-- +goose Up
CREATE TABLE tenants (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## 3.2 clusters
```sql
-- +goose Up
CREATE TABLE clusters (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_clusters_tenant ON clusters(tenant_id);
```

## 3.3 nodes
```sql
-- +goose Up
CREATE TABLE nodes (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  role TEXT NOT NULL CHECK(role IN ('admin','node')),
  token_hash TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_nodes_token_hash ON nodes(token_hash);
CREATE INDEX idx_nodes_cluster ON nodes(cluster_id);
```

## 3.4 config_bundles
```sql
-- +goose Up
CREATE TABLE config_bundles (
  version INTEGER NOT NULL,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  data BLOB NOT NULL,
  created_by TEXT NOT NULL REFERENCES nodes(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (tenant_id, cluster_id, version)
);
```

---

# 4. Go Server Layout

Standard Go project layout:

```
/server
  /cmd
    /nebulagc-server   # Cobra root; subcommands: serve, tenant/cluster/node ops
      main.go          # Entry point, wiring
  /internal
    /api               # HTTP Handlers & Router
      router.go
      middleware.go    # Auth middleware
      handlers.go
    /auth              # Token hashing & validation logic
    /db                # SQLc generated code
      models.go
      query.sql.go
    /service           # Business logic (if needed beyond handlers)
  /migrations          # Goose SQL files
    001_init.sql
  sqlc.yaml            # SQLc config
  go.mod
```

### Key Libraries:
- Router: `github.com/gin-gonic/gin`
- DB Driver: `modernc.org/sqlite`
- Migrations: `github.com/pressly/goose/v3`
- SQL Generation: `github.com/sqlc-dev/sqlc`
- Logging: `log/slog` with JSON handler (stdout)
- Metrics: optional `/metrics` (Prometheus) served on a separate port or under `/v1/metrics` if behind auth
- CLI: `github.com/spf13/cobra` for server and daemon bins
- TUI tables: `github.com/charmbracelet/bubbletea` + `bubbles/table` for human-readable output; wrap with `--output json` flag for machine use

### Auth Middleware Logic
1. Extract headers: `X-Nebula-Node-ID`, `X-Nebula-Node-Token`.
2. Look up node by `id` in DB.
3. Compare `token_hash` in DB with `hash(header_token)`.
4. If match, inject `Node` object into context.
5. If fail, return 401.

### Bootstrap Flow
- A one-time `bootstrap_admin_token` is provided via env var `NEBULA_BOOTSTRAP_TOKEN`. If set, the server CLI (not REST) accepts `--bootstrap-token` for the very first tenant and cluster admin creation.
- After an admin node exists, disable the bootstrap path (or require env var to be unset).

### Server Cobra Commands (examples)
- `nebulagc-server serve --http :8080 --db ./nebula.db`
- `nebulagc-server tenant create --name Acme --bootstrap-token $NEBULA_BOOTSTRAP_TOKEN`
- `nebulagc-server cluster create --tenant-id ... --name prod`
- `nebulagc-server node create --tenant-id ... --cluster-id ... --name admin-1 --role admin`
- `nebulagc-server list tenants|clusters|nodes [--tenant-id ... --cluster-id ...] [--output json]`
- Add `--admin-socket /var/run/nebulagc_admin.sock` to enable the Unix socket control plane; commands go over the socket when provided.

---

# 5. Go Client SDK (Cluster-Scoped)

The Go SDK hides headers and JSON handling.

```
sdk/
  client.go
  types.go
  errors.go
```

Main interface:

```go
type Client struct {
    BaseURL    string
    TenantID   string
    ClusterID  string
    NodeID     string
    NodeToken  string
    HTTPClient *http.Client
}

func NewClient(baseURL, tenantID, clusterID, nodeID, nodeToken string) *Client

func (c *Client) GetLatestVersion(ctx context.Context) (int, error)
func (c *Client) DownloadBundle(ctx context.Context, current int) ([]byte, int, error)
func (c *Client) UploadBundle(ctx context.Context, data []byte) (int, error)
```

---

# 6. nebulagc CLI / Daemon (Cluster Admin)

`nebulagc` is a high-level daemon that manages one or more underlying `nebula` processes. It supports connecting to multiple clusters simultaneously by running separate `nebula` instances for each configuration. It exposes *cluster-admin* functionality only (no cross-tenant listing).

### Features
- **Multi-Cluster Support**: Manages multiple `nebula` processes in parallel.
- **Configuration Management**: Reads `/etc/nebulagc/config.json` to know which clusters to join.
- **Auto-Updates**: Polls the control plane every 5 seconds for each cluster.
  - If a new version is found, it downloads the bundle, updates the local config, and restarts the specific `nebula` process.
- **Process Supervision**: Ensures `nebula` processes are running and restarts them if they crash.
- **Output Modes**: Bubble Tea table views by default for status/list commands; `--output json` for automation.

### Configuration: `/etc/nebulagc/config.json`

This file defines the clusters this node should join.

```json
{
  "clusters": [
    {
      "name": "prod-eu-west",
      "control_plane_url": "https://api.nebulagc.example.com/v1",
      "tenant_id": "uuid-tenant-1",
      "cluster_id": "uuid-cluster-1",
      "node_id": "uuid-node-1",
      "node_token": "secret-token-1",
      "config_dir": "/etc/nebula/prod-eu-west"
    },
    {
      "name": "management-net",
      "control_plane_url": "https://api.nebulagc.example.com/v1",
      "tenant_id": "uuid-tenant-1",
      "cluster_id": "uuid-cluster-2",
      "node_id": "uuid-node-2",
      "node_token": "secret-token-2",
  "config_dir": "/etc/nebula/management-net"
    }
  ]
}
```

### Local Development Config
Provide a `dev_config.json` in repo root that points to `http://127.0.0.1:8080/v1` and places bundle contents under `./tmp/<cluster>`. Git-ignore `tmp/` and `dev_config.json`.

### Daemon Logic

1. **Startup**: Read `/etc/nebulagc/config.json`.
2. **Spawn**: For each cluster entry:
   - Create a `Manager` routine.
   - Ensure `config_dir` exists.
   - Start a poller loop.
3. **Poller Loop** (per cluster):
   - Check `GetLatestVersion()`.
   - If new version > current version:
     - `DownloadBundle()`.
     - Unpack to `config_dir`.
     - Restart `nebula` process pointing to `config_dir/config.yml`.
   - If `nebula` process is not running, start it:
     - `nebula -config /etc/nebula/<name>/config.yml`

### Deployment

#### Systemd

Run `nebulagc` as a system service. It will spawn child `nebula` processes.

`/etc/systemd/system/nebulagc.service`:

```ini
[Unit]
Description=NebulaGC Daemon
After=network.target

[Service]
ExecStart=/usr/local/bin/nebulagc daemon --config /etc/nebulagc/config.json
Restart=always
User=root
Group=root

[Install]
WantedBy=multi-user.target
```

#### Docker

Run `nebulagc` as the entrypoint. It will manage `nebula` processes inside the container.

`Dockerfile`:

```dockerfile
FROM alpine:latest

# Install nebula binary
COPY --from=nebula-build /usr/bin/nebula /usr/bin/nebula
# Install nebulagc binary
COPY bin/nebulagc /usr/bin/nebulagc

# Create config dirs
RUN mkdir -p /etc/nebulagc /etc/nebula

ENTRYPOINT ["/usr/bin/nebulagc", "daemon", "--config", "/etc/nebulagc/config.json"]
```

---

# 7. Super Admin Interface (Server Cobra CLI + Unix Socket)

Super-admin operations are only available via the server’s Cobra CLI and an optional privileged Unix socket. REST remains cluster-scoped and cannot list tenants/clusters across the system.

### Capabilities (super admin only)
- `tenant create --name ...`
- `cluster create --tenant-id ... --name ...`
- `node create --tenant-id ... --cluster-id ... --name ... --role admin|node` (for initial admin distribution)
- `tenant list` / `cluster list` / `node list --tenant-id ... --cluster-id ...`
- Rotate Nebula PKI for a cluster by generating a new bundle in the cluster’s folder.

### Output modes
- Default: Bubble Tea table views for human-readable output.
- `--output json` flag to emit machine-readable JSON (both for the server CLI and the `nebulagc` daemon CLI).

### Unix socket interface
- Expose the same super-admin commands over a local Unix socket (e.g. `/var/run/nebulagc_admin.sock`), gated by filesystem permissions. The socket server can be reused by other codebases for tight integrations.
- Disable the socket by default; enable via `NEBULAGC_ADMIN_SOCKET_PATH` env var or CLI flag.

### Artifact layout on new tenant/cluster creation
- Creating a cluster via the server CLI auto-creates a folder (e.g. `/var/lib/nebulagc/<tenant>/<cluster>/`) containing Nebula config files and certificates.
- The CLI can optionally emit a tarball to hand over to the tenant admin.

---

# 8. Development & Build

## Prerequisites
- Go 1.22+
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `go install github.com/pressly/goose/v3/cmd/goose@latest`

## Workflow

1. **Modify Schema**: Edit `server/migrations/*.sql`.
2. **Generate DB Code**:
   ```bash
   cd server
   sqlc generate
   ```
3. **Run Migrations** (Dev):
   ```bash
   goose -dir server/migrations sqlite3 ./nebula.db up
   ```
4. **Build Control Plane**:
   ```bash
   go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
   ```
5. **Build NebulaGC Daemon**:
   ```bash
   go build -o bin/nebulagc ./cmd/nebulagc
   ```

## Running Locally

```bash
# Start server
./bin/nebulagc-server

# Start daemon (in another terminal)
# Ensure config.json exists at ./config.json or specify path
./bin/nebulagc daemon --config ./dev_config.json
```

## Recommended Extras
- `make format` → `gofmt ./...`
- `make lint` → `golangci-lint run ./...` (if available)
- `make test` → `go test ./...`
- Add a `docker-compose.yml` for local smoke tests with an in-container SQLite file and port 8080 exposed.
