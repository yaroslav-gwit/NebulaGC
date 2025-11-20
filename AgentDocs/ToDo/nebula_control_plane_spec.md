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

# Phased Delivery Plan

**Phase 1: HA control plane core**
- Master/replica runtime, replica registry/heartbeat, and write guards for REST/CLI/socket.
- Cluster/node CRUD, token hashing, bundle upload/download with version headers, config_version tracking.
- Lighthouse watcher on all instances with PKI stored in DB.
- Goose migrations + SQLc models; basic Bubble Tea/JSON output for CLI and socket.

**Phase 2: Node-facing SDK and daemon**
- Go SDK covering all cluster-scoped REST endpoints (reads and writes with master discovery).
- `nebulagc` daemon: multi-cluster polling, bundle unpack/restart, HA failover using `/v1/replicas` + `/v1/check-master`.
- Route management, MTU updates, lighthouse/relay flags, token rotation flows propagated to bundles.

**Phase 3: Ops hardening and tooling**
- Rate limiting, structured logging, auth-failure telemetry.
- Systemd/Docker deployment docs, dev tooling (make targets, lint/test), sample dev config.
- Cleanup utilities: replica pruning, bundle verification, cross-instance lighthouse health checks.

# 1. Concepts & Requirements

## 1.0 High Availability (HA) Architecture

### Master/Replica Mode
The control plane supports HA deployment with one master and multiple read-only replicas:

**Master Instance:**
- Started with `--master` flag (required, no default)
- Handles all write operations (node creation, config uploads, topology changes)
- Only master accepts mutating commands from REST, the admin Unix socket, or the Cobra CLI; replicas must fail writes.
- Exposes `GET /v1/check-master` endpoint returning `{"master": true}`
- Manages lighthouse processes for clusters with `provide_lighthouse=true`
- Tracks cluster state versions and manages lighthouse lifecycle

**Replica Instances:**
- Started with `--replicate` flag (required, no default)
- Read-only: serves config downloads, version checks, topology queries
- Returns errors for write operations (POST/PATCH/DELETE) with `503 SERVICE_UNAVAILABLE` and message: `{"error": "This is a read-only replica. Write to master.", "code": "REPLICA_READ_ONLY"}`
- Local CLI/admin socket attempts to mutate state must fail fast with clear “replica is read-only; run on master” errors.
- Exposes `GET /v1/check-master` endpoint returning `{"master": false}`
- **Also manages lighthouse processes** for improved availability
- Tracks cluster state versions and manages lighthouse lifecycle independently

**Startup Requirements:**
- MUST specify either `--master` or `--replicate` flag
- Starting without either flag results in immediate exit with error message
- Cannot specify both flags simultaneously

**SQLite Replication:**
- User manages SQLite replication externally (e.g., Litestream, LiteFS, custom solution)
- Control plane assumes replicas have eventually consistent read access to database
- Master writes, replicas read

### Cluster State Management
All instances (master and replicas) track cluster configuration state:

**State Tracking:**
- Each cluster has a `config_version` (incremented on PKI changes, lighthouse changes, node topology changes)
- `running_config_version` stored in memory and persisted to DB per cluster
- Background goroutine runs every 5 seconds on **all instances** (master and replicas)
- Checks if `running_config_version` < `config_version` in DB
- If stale, re-evaluates lighthouse requirements and starts/restarts Nebula lighthouse processes

**Lighthouse Process Management:**
- **Both master and replicas** spawn lighthouse processes for clusters with `provide_lighthouse=true`
- This provides N-way lighthouse redundancy (all control plane instances serve as lighthouses)
- If a replica detects config version change, it restarts its lighthouse processes with new config
- Lighthouse discovery: clients can query any instance's `/v1/replicas` to get all control plane addresses

### Replica Discovery
Clients can discover all control plane instances:

**Discovery Mechanism:**
- New DB table: `replicas` containing all control plane instance addresses
- Endpoint: `GET /v1/replicas` returns list of all instances (master + replicas)
- Clients should:
  1. Query `/v1/replicas` from any known instance
  2. For writes, query each instance's `/v1/check-master` to find the master
  3. Use any instance for reads (config downloads, version checks)
  4. Use all instances as lighthouses (if they provide lighthouse for the cluster)

### Replica Registry Maintenance
The `replicas` table is kept fresh automatically:
- On startup, every instance registers (or upserts) its `id`, `address`, and role into `replicas`.
- Master validates that a single `role=master` row exists; startup fails if multiple masters are registered.
- A background heartbeat on **all instances** updates `last_seen_at` every 30 seconds.
- Stale entries (no heartbeat for >5 minutes) are pruned by the master to avoid advertising dead replicas.
- Replica discovery endpoints read directly from this table; SDK/daemon use it for failover and lighthouse lists.

## 1.1 Core entities

### Tenant
Represents a customer / organisation.  
Each tenant can contain one or more **clusters**.

### Cluster  
A logical Nebula environment for a tenant (e.g. `prod`, `dev`, `eu-west`, `lab`).

**Configuration versioning:**
- `config_version` (integer): Incremented on PKI changes, lighthouse topology changes, or node changes
- `running_config_version` (integer): Last version applied by the lighthouse manager (tracked per instance)
- Version mismatch triggers lighthouse process restart

**Optional lighthouse functionality:**
- `provide_lighthouse` (boolean): If true, all control plane instances (master + replicas) act as lighthouses for this cluster
- `lighthouse_port` (integer): UDP port for lighthouse traffic (typically 4242)
- Useful for setups without public IPs where the control plane itself can serve as a lighthouse
- **Implementation:** When `provide_lighthouse=true`, each control plane server (master and all replicas) spawns a dedicated Nebula process configured as a lighthouse for this cluster
  - Each cluster with lighthouse enabled gets its own Nebula instance on **each control plane server**
  - This provides N-way lighthouse redundancy where N = number of control plane instances
  - Lighthouse instance uses the cluster's lighthouse_port and proper certificates from the cluster's PKI
  - Control plane manages these Nebula lighthouse processes alongside the HTTP API server
  - Background goroutine checks every 5 seconds if `running_config_version` is stale and restarts lighthouse processes

**Certificate storage:**
- Cluster PKI (CA cert, CRL, signing key) stored directly in database
- Enables any control plane instance to generate node certificates or serve as lighthouse
- No filesystem dependency for certificate distribution

### Node  
A machine that:

- belongs to exactly **one tenant**
- belongs to exactly **one cluster**
- is authenticated by:
  - `node_id` (UUID)
  - `node_token` (minimum 41 characters, randomly generated secret)
  - `tenant_id` (UUID)
  - `cluster_id` (UUID)
  - `cluster_token` (minimum 41 characters, shared secret for all nodes in a cluster)
- has an `is_admin` flag (boolean):
  - `true` → cluster admin, may create nodes in its own cluster and upload config bundles for that cluster
  - `false` → regular node, may only fetch config for its cluster
- can be designated as **lighthouse** or **relay** (admin-only operation):
  - `is_lighthouse` (boolean): If true, this node acts as a lighthouse for the cluster
  - `is_relay` (boolean): If true, this node acts as a relay for the cluster
  - A node can be both lighthouse and relay simultaneously
  - Only admin nodes can assign/unassign lighthouse or relay status
  - Lighthouse nodes help with NAT traversal and node discovery
  - Relay nodes forward traffic for nodes that cannot establish direct connections
- has configurable **MTU** (Maximum Transmission Unit):
  - `mtu` (integer): MTU size in bytes for this node's Nebula interface
  - Defaults to 1300 bytes (safe for most networks)
  - Can be adjusted per-node for optimal performance based on network conditions
  - Valid range: 1280-9000 bytes (IPv6 minimum to jumbo frames)
  - MTU changes propagated via config bundle updates
- can advertise **internal routes** (IP subnets) to the cluster:
  - All nodes (admin or regular) can register their internal routes
  - Useful for routers that need to expose internal networks to other nodes
  - Routes are propagated via config bundles to all nodes in the cluster

### Cluster Token
A shared secret known by all nodes (both admin and regular) in a cluster. This provides an additional authentication layer:
- Generated when a cluster is created
- Shared with all nodes during enrollment
- Acts as a cluster-level password in addition to per-node credentials
- Must be included in all API requests via the `X-Nebula-Cluster-Token` header
- Stored as `cluster_token_hash` in the database (never in plaintext)

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
- Super admin uses the server Cobra CLI (or Unix socket) to create a tenant and its initial cluster.
- Super admin creates the first admin node for the cluster via CLI, receiving both `node_token` and `cluster_token`.
- Super admin hands credentials (`tenant_id`, `cluster_id`, `node_id`, `node_token`, `cluster_token`) to the tenant admin.
- Tenant admin runs `nebulagc` using those admin credentials to enroll additional nodes over the REST API (cluster-scoped).
- Super admin can list tenants/clusters/nodes via the CLI or Unix socket to verify posture; this visibility is not exposed over REST.

### Config Version Lifecycle & Lighthouse Synchronization

**Operations that increment `config_version`:**
- Node topology changes (lighthouse/relay assignment)
- MTU updates for nodes
- PKI rotation (manual via CLI)
- Node route changes (depending on implementation)
- Node creation or deletion (new certs / revocation)
- Node token rotation and cluster token rotation (mark the window when new secrets must be deployed)

**After `config_version` increment:**
1. Master writes new `config_version` to `clusters` table
2. SQLite replication propagates change to all replicas (user-managed, typically < 1 second)
3. Each control plane instance's lighthouse watcher goroutine (runs every 5s) detects:
   - `clusters.config_version > cluster_state.running_config_version` for this instance
4. Each instance independently:
   - Reads cluster PKI from database (`pki_ca_cert`, `pki_ca_key`, `pki_crl`)
   - Queries `/v1/replicas` to get all control plane addresses (for lighthouse list)
   - Regenerates Nebula lighthouse config file with updated topology
   - Restarts its Nebula lighthouse process for this cluster
   - Updates `cluster_state.running_config_version = config_version` for its `instance_id`
5. Result: All control plane instances (master + N replicas) serve as synchronized lighthouses within ~5 seconds

**Client nodes discover lighthouses via:**
- Download config bundle from any control plane instance
- Bundle includes all control plane instance addresses (from `/v1/replicas`)
- Nebula nodes connect to all available lighthouses for redundancy

## 1.3 Token Security Requirements

### Token Generation
- All tokens (`node_token`, `cluster_token`) must be cryptographically random, minimum 41 characters
- Recommended: Generate 32 bytes (256 bits) of random data, base64-encode (44 chars), or use UUID + random suffix
- Tokens are secrets and must never be logged in production except when explicitly debugging authentication failures

### Token Storage
- Database stores only `token_hash` (HMAC-SHA256) using a server-side secret key
- HMAC key should be configured via `NEBULAGC_HMAC_SECRET` environment variable (minimum 32 bytes)
- Never return tokens after initial creation, except `node_token` in create-node response
- `cluster_token` is returned once when cluster is created, and again when admin nodes are created for that cluster

### Token Rotation
- Cluster admins must be able to rotate **node tokens** and the shared **cluster token** from the REST API.
- Rotation invalidates the previous token immediately and forces re-auth on the next request.
- Rotation increments `config_version` so lighthouse watchers and bundles reflect the change window; nodes must refresh tokens out-of-band (admin distributes new token to affected nodes).
- Rotation events are logged at INFO with node/cluster identifiers (without logging the new token).

---

# 2. HTTP API (Cluster-Scoped)

Base URL: `/v1`

All requests must send the following headers unless noted otherwise:

- `X-Nebula-Tenant-ID` (UUID of the tenant)
- `X-Nebula-Cluster-ID` (UUID of the cluster)
- `X-Nebula-Node-ID` (UUID of the calling node)
- `X-Nebula-Node-Token` (minimum 41 character secret, unique per node)
- `X-Nebula-Cluster-Token` (minimum 41 character secret, shared by all nodes in cluster)

## Error Handling

Errors are returned in JSON format with a non-2xx status code:

```json
{
  "error": "Authentication failed",
  "code": "UNAUTHORIZED" 
}
```

Common error codes:
- `UNAUTHORIZED` (401): Authentication failed - invalid credentials or non-existent tenant/cluster/node
- `FORBIDDEN` (403): Node lacks required role (e.g., non-admin trying admin operation)
- `NOT_FOUND` (404): Resource not found (after successful authentication)
- `BAD_REQUEST` (400): Invalid request format, missing required fields, or bundle validation failure
- `CONFLICT` (409): Resource already exists
- `PAYLOAD_TOO_LARGE` (413): Bundle exceeds 10 MiB limit
- `RATE_LIMIT_EXCEEDED` (429): Too many requests, includes `Retry-After` header (seconds)
- `REPLICA_READ_ONLY` (503): Write operation attempted on read-only replica, client should retry on master
- `INTERNAL_ERROR` (500): Server-side failure

### Authentication Error Responses

For security, all authentication failures return a generic `UNAUTHORIZED` response:

```json
{
  "error": "Authentication failed",
  "code": "UNAUTHORIZED"
}
```

This applies to:
- Invalid node_id
- Invalid node_token
- Invalid cluster_token
- Non-existent tenant_id or cluster_id
- Mismatched tenant/cluster/node combinations

**Server-side logging:** While the response is generic, the server MUST log the specific failure reason at WARN level with full details:
- Timestamp
- Source IP
- All provided headers (including tokens in cleartext)
- Specific failure reason (e.g., "node_id not found", "node_token mismatch", "cluster_token invalid")
- Request path and method

This enables administrators to distinguish fat-finger configuration errors from malicious brute-force attempts.

### Auth & Security Notes

#### Token Handling
- All tokens are secrets with minimum 41 character length
- `node_token` is returned only at node creation
- `cluster_token` is returned when cluster is created and when admin nodes are created
- Store `token_hash` (HMAC-SHA256) in DB using server secret from `NEBULAGC_HMAC_SECRET` env var
- Never store raw tokens in the database

#### Authorization
- Cluster-admin-only routes require the calling node to have `is_admin=1` (true) in DB *and* match `(tenant_id, cluster_id)` in headers
- All requests must provide valid `cluster_token` matching the cluster's stored hash
- Super-admin actions (tenant/cluster lifecycle, global listing) are **not** available via REST; only via server CLI or the privileged Unix socket interface
- All nodes (admin or regular) can register and view internal routes

#### Rate Limiting
- Implement per-IP rate limiting with a token bucket algorithm (in-memory is fine for now)
- Aggressive limits for authentication failures:
  - 10 failed auth attempts per IP per minute → 429 with `Retry-After: 60`
  - After 50 failed attempts in 10 minutes → 1 hour block
- Lenient limits for authenticated requests (legitimate polling):
  - 100 requests per node per minute for config checks
  - 10 bundle uploads per cluster per minute
- Rate limit state should be keyed by:
  - IP address for unauthenticated/failed attempts
  - `node_id` for successfully authenticated requests

#### Health Check
- Minimal unauthenticated `GET /v1/healthz` returns `200 OK` for liveness probes
- No database access, no authentication required
- Returns: `{"status": "ok"}`

#### Master Detection
- Unauthenticated `GET /v1/check-master` returns master status
- No database access, no authentication required
- Master returns: `{"master": true}`
- Replica returns: `{"master": false}`
- Clients use this to discover which instance accepts writes

#### Replica Discovery
- Unauthenticated `GET /v1/replicas` returns all control plane instances
- Returns: `{"instances": [{"address": "https://control1.example.com", "role": "master"}, {"address": "https://control2.example.com", "role": "replica"}]}`
- Clients should query any instance to discover full cluster topology
- Use for both API requests and lighthouse discovery

### Scope & Anon-Friendly Rules
- The REST API is cluster-scoped: callers can only manage nodes and config for the cluster in their headers.
- **Write operations on replicas:** All write endpoints (POST/PATCH/DELETE) return `503 SERVICE_UNAVAILABLE` with `REPLICA_READ_ONLY` error code when called on a replica instance. Clients must send writes to the master.
- Nodes with `is_admin=true` are *cluster admins*, not super admins. They can only:
  - Create nodes within their own `(tenant, cluster)`.
  - Upload/download config bundles for their cluster.
  - Rotate Nebula certificates for their cluster (by uploading a new bundle).
  - Assign/unassign lighthouse and relay status to nodes in their cluster.
  - Update MTU settings for nodes in their cluster.
  - Rotate node tokens and the shared cluster token for compromise recovery.
  - Delete nodes and list all nodes in their cluster.
- All nodes (admin or regular) can:
  - Register and update their internal routes.
  - View all routes in their cluster.
  - View cluster topology (lighthouses and relays).
  - Download config bundles and check for updates.
- Tenants and clusters cannot be listed or created over REST. Super-admin functions live in the server CLI or a privileged Unix socket API (see Section 7).

## 2.1 Create Node (Cluster Admin Only)
**POST `/v1/tenants/{tenant_id}/clusters/{cluster_id}/nodes`**

Headers: cluster-scoped auth as usual; the caller must have `role=admin` for this cluster.

Body:
```json
{
  "name": "hoster-node-01",
  "is_admin": false,
  "mtu": 1300
}
```

**Parameters:**
- `name` (string, required): Human-readable node name
- `is_admin` (boolean, required): Admin status
- `mtu` (integer, optional): MTU size in bytes (default: 1300, range: 1280-9000)

Response (201 Created):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "node_token": "<41+ character random secret>",
  "cluster_token": "<41+ character cluster secret>",
  "created_at": "2025-11-20T10:30:00Z"
}
```

The `node_token` is returned **only at creation** and cannot be retrieved later. The `cluster_token` is included here for convenience (same token for all nodes in the cluster). Store both securely.

**Side effects:**
- Increments the cluster `config_version` (new node cert + topology) which triggers lighthouse restarts on all instances.

## 2.2 Update Node MTU (Admin Only)
**PATCH `/v1/nodes/{node_id}/mtu`**

Update the MTU for a specific node. Only admin nodes can perform this operation.

Body:
```json
{
  "mtu": 1400
}
```

**Parameters:**
- `mtu` (integer, required): MTU size in bytes (range: 1280-9000)

**Validation:**
- Returns `400 BAD_REQUEST` if MTU < 1280 or MTU > 9000
- Returns `403 FORBIDDEN` if calling node is not admin

Response (200 OK):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "hoster-node-01",
  "mtu": 1400,
  "updated_at": "2025-11-20T10:33:00Z"
}
```

**Notes:**
- MTU change is included in the next config bundle
- Triggering a bundle download and node restart applies the new MTU
- Increments the cluster `config_version` to propagate through lighthouse restarts and bundles

## 2.3 Rotate Node Token (Admin Only)
**POST `/v1/nodes/{node_id}/token`**

Rotates a node's secret. Only admin nodes can perform this operation.

Response (200 OK):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "node_token": "<new 41+ char secret>",
  "rotated_at": "2025-11-20T10:40:00Z"
}
```

**Notes:**
- Old token stops working immediately.
- Increments the cluster `config_version` to mark the rotation window for audit and cache invalidation.
- New token is returned once; never logged beyond WARN-level auth failures.

## 2.4 Delete Node (Admin Only)
**DELETE `/v1/nodes/{node_id}`**

Removes a node from the cluster, revoking its access.

Response:
- `204 No Content` on success
- `404 NOT_FOUND` if node does not exist after auth

**Side effects:**
- Increments the cluster `config_version`.
- Removes any lighthouse/relay flags and routes associated with the node to keep bundles clean.
- Existing bundle history keeps the version but `created_by` may be null after deletion.

## 2.5 List Cluster Nodes (Admin Only)
**GET `/v1/nodes`**

Lists nodes scoped to the authenticated `(tenant_id, cluster_id)`.

Response (200 OK):
```json
{
  "cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "nodes": [
    {
      "node_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "admin-1",
      "is_admin": true,
      "mtu": 1300,
      "is_lighthouse": true,
      "is_relay": false,
      "routes": ["10.20.0.0/16"],
      "created_at": "2025-11-20T10:30:00Z",
      "updated_at": "2025-11-20T10:45:00Z"
    }
  ]
}
```

**Notes:**
- Does **not** return tokens.
- Supports pagination params `?page` and `?page_size` (default `page_size=50`, max 500).
- Useful for cluster admins without requiring super-admin CLI access.

## 2.6 Register Internal Routes (Any Node)
**POST `/v1/routes`**

Allows any node (admin or regular) to advertise its internal IP subnets/routes to the cluster. Useful for routers, gateways, or nodes that need to expose internal networks.

Body:
```json
{
  "routes": [
    "10.20.0.0/16",
    "192.168.100.0/24"
  ]
}
```

Response (200 OK):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "routes": [
    "10.20.0.0/16",
    "192.168.100.0/24"
  ],
  "updated_at": "2025-11-20T10:32:00Z"
}
```

**Notes:**
- Routes are stored per-node and included in the next config bundle generation
- Empty array clears all routes for this node
- Routes are validated as valid CIDR notation (IPv4 or IPv6)
- Invalid CIDR returns `400 BAD_REQUEST`
- Increments the cluster `config_version` to trigger bundle regeneration and lighthouse restarts

## 2.7 Get Node Routes (Any Node)
**GET `/v1/routes`**

Retrieve the current node's registered routes.

Response (200 OK):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "routes": [
    "10.20.0.0/16",
    "192.168.100.0/24"
  ],
  "updated_at": "2025-11-20T10:32:00Z"
}
```

## 2.8 List All Cluster Routes (Any Node)
**GET `/v1/routes/all`**

Retrieve all routes advertised by all nodes in the cluster. Useful for debugging routing issues.

Response (200 OK):
```json
{
  "cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "nodes": [
    {
      "node_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "router-node-01",
      "routes": ["10.20.0.0/16", "192.168.100.0/24"],
      "updated_at": "2025-11-20T10:32:00Z"
    },
    {
      "node_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "name": "gateway-node-02",
      "routes": ["172.16.0.0/12"],
      "updated_at": "2025-11-20T09:15:00Z"
    }
  ]
}
```

## 2.9 Set Node Lighthouse Status (Admin Only)
**POST `/v1/nodes/{node_id}/lighthouse`**

Designate a node as a lighthouse. Only admin nodes can perform this operation.

Body:
```json
{
  "is_lighthouse": true,
  "public_ip": "203.0.113.42",
  "lighthouse_port": 4242
}
```

**Parameters:**
- `is_lighthouse` (boolean, required): Enable or disable lighthouse status
- `public_ip` (string, optional): Public IP address for the lighthouse (required if `is_lighthouse=true`)
- `lighthouse_port` (integer, optional): UDP port (defaults to cluster's lighthouse_port or 4242)

Response (200 OK):
```json
{
  "node_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "lighthouse-01",
  "is_lighthouse": true,
  "public_ip": "203.0.113.42",
  "lighthouse_port": 4242,
  "updated_at": "2025-11-20T10:45:00Z"
}
```

**Notes:**
- Setting `is_lighthouse=false` removes lighthouse status
- Lighthouse configuration is included in next config bundle
- Returns `403 FORBIDDEN` if calling node is not admin
- Increments the cluster `config_version` to notify lighthouse managers on all instances

## 2.10 Set Node Relay Status (Admin Only)
**POST `/v1/nodes/{node_id}/relay`**

Designate a node as a relay. Only admin nodes can perform this operation.

Body:
```json
{
  "is_relay": true
}
```

Response (200 OK):
```json
{
  "node_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "relay-01",
  "is_relay": true,
  "updated_at": "2025-11-20T10:46:00Z"
}
```

**Notes:**
- A node can be both lighthouse and relay
- Relay configuration is included in next config bundle
- Returns `403 FORBIDDEN` if calling node is not admin
- Increments the cluster `config_version` so relay changes propagate through bundles and lighthouse restarts

## 2.11 List Cluster Lighthouses and Relays (Any Node)
**GET `/v1/topology`**

Retrieve cluster topology: all lighthouses and relays.

Response (200 OK):
```json
{
  "cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "provide_lighthouse": true,
  "control_plane_lighthouse": {
    "enabled": true,
    "host": "lighthouse.nebulagc.example.com",
    "port": 4242
  },
  "lighthouses": [
    {
      "node_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "lighthouse-01",
      "public_ip": "203.0.113.42",
      "port": 4242,
      "is_relay": false
    }
  ],
  "relays": [
    {
      "node_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "name": "relay-01",
      "is_lighthouse": false
    },
    {
      "node_id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "lighthouse-01",
      "is_lighthouse": true
    }
  ]
}
```

**Notes:**
- `control_plane_lighthouse` is included if cluster has `provide_lighthouse=true`
- Nodes can appear in both lighthouses and relays arrays
- This data is used to generate proper Nebula config files

## 2.12 Get Latest Config Version (Any Node)
**GET `/v1/config/version`**

Returns:
```json
{
  "latest_version": 4
}
```

## 2.13 Download Config Bundle (Any Node)
**GET `/v1/config/bundle?current_version=3`**

Response:
- `304 Not Modified` → nothing changed
- `200 OK` → with `X-Nebula-Config-Version: <int>` and binary body (tar.gz)

## 2.14 Upload Config Bundle (Admin Node)
**POST `/v1/config/bundle`**

Binary body (tar.gz).  
Server increments version automatically using **last-write-wins** strategy.

**Concurrent Upload Handling:**
- If multiple admins upload simultaneously, both succeed
- Version numbers increment atomically: version 3 → 4, then 4 → 5
- Version numbers may skip if bundles are deleted/corrupted (no gaps guaranteed)
- No optimistic locking; application should handle this through operational procedures

Returns (200 OK):
```json
{ 
  "version": 5,
  "uploaded_at": "2025-11-20T10:35:00Z"
}
```

## 2.15 Rotate Cluster Token (Admin Only)
**POST `/v1/cluster/token`**

Rotates the shared cluster secret. Only admin nodes can perform this operation.

Response (200 OK):
```json
{
  "cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "cluster_token": "<new 41+ char secret>",
  "rotated_at": "2025-11-20T10:50:00Z"
}
```

**Notes:**
- Old cluster token stops authenticating immediately.
- Increments `config_version` to mark the rotation window; bundle downloads include the new version header to help daemons coordinate.
- Admin must redistribute the new cluster token to all nodes out-of-band (SDK/daemon config).

## 2.16 Config Bundle Contract

### Required Files
The archive must contain:
- `config.yml` - The Nebula config the process will read
- `ca.crt` - Certificate Authority certificate
- `crl.pem` - Certificate Revocation List
- `host.crt` - Host certificate
- `host.key` - Host private key
- `lighthouses.json` - Optional helper list (daemon can derive from config)

### Validation Rules
1. **Size limit:** Maximum 10 MiB (10,485,760 bytes)
   - Reject with `413 PAYLOAD_TOO_LARGE` if exceeded

2. **Archive format:** Must be valid gzip-compressed tar archive
   - Reject with `400 BAD_REQUEST` if corrupt or wrong format

3. **Required files:** Must contain all required files listed above (except `lighthouses.json`)
   - Reject with `400 BAD_REQUEST` if missing, with error: `{"error": "Missing required file: <filename>", "code": "BAD_REQUEST"}`

4. **Content validation:** YAML syntax check for `config.yml`
   - Reject with `400 BAD_REQUEST` if invalid YAML: `{"error": "Invalid YAML in config.yml", "code": "BAD_REQUEST"}`
   - Do not validate semantic correctness (lighthouses, ports, etc.) - that's Nebula's job

5. **Storage:** Server stores bundles verbatim after validation

### Bundle Generation & Routes
When generating a config bundle:
- **Any control plane instance can generate bundles** (master or replica) since PKI is stored in database
- Include all registered internal routes from all nodes in the cluster
- Include per-node MTU settings in each node's config section
- Include lighthouse configuration:
  - **All control plane instances** (if `provide_lighthouse=true` for cluster) - query `/v1/replicas` for addresses
  - All nodes with `is_lighthouse=true`, including their public IPs and ports
- Include relay configuration:
  - All nodes with `is_relay=true`
- Routes can be included in `lighthouses.json` or embedded in `config.yml` depending on implementation
- Each node's routes are tagged with its node_id for reference
- Empty routes array for a node means no routes advertised
- Node lifecycle changes, route updates, token rotations, lighthouse/relay status changes, or MTU updates trigger `config_version` increment
- `config_version` increment triggers lighthouse process restarts on all instances

### Response Headers
- Successful download (200 OK) must include: `X-Nebula-Config-Version: <integer>`
- Not modified (304) includes same header with current version

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
  cluster_token_hash TEXT NOT NULL,
  provide_lighthouse INTEGER NOT NULL DEFAULT 0 CHECK(provide_lighthouse IN (0,1)),
  lighthouse_port INTEGER DEFAULT 4242,
  config_version INTEGER NOT NULL DEFAULT 1, -- Incremented on PKI/topology changes
  pki_ca_cert TEXT, -- PEM-encoded CA certificate
  pki_ca_key TEXT, -- PEM-encoded CA private key (encrypted)
  pki_crl TEXT, -- PEM-encoded Certificate Revocation List
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(tenant_id, name)
);

CREATE INDEX idx_clusters_tenant ON clusters(tenant_id);

-- Separate table to track running config version per control plane instance
CREATE TABLE cluster_state (
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  instance_id TEXT NOT NULL, -- Control plane instance ID
  running_config_version INTEGER NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (cluster_id, instance_id)
);

CREATE INDEX idx_cluster_state_version ON cluster_state(cluster_id, running_config_version);
```

## 3.3 replicas
```sql
-- +goose Up
CREATE TABLE replicas (
  id TEXT PRIMARY KEY,
  address TEXT NOT NULL UNIQUE, -- Full URL: https://control1.example.com
  role TEXT NOT NULL CHECK(role IN ('master','replica')),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at DATETIME
);

CREATE INDEX idx_replicas_role ON replicas(role);
```

**Notes:**
- `id`: Unique identifier for this control plane instance
- `address`: Full URL including protocol and port
- `role`: Either 'master' or 'replica'
- `last_seen_at`: Updated by health checks (future enhancement)
- Master is responsible for maintaining this table (manual or via CLI)

## 3.4 nodes
```sql
-- +goose Up
CREATE TABLE nodes (
  id TEXT PRIMARY KEY,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  is_admin INTEGER NOT NULL DEFAULT 0 CHECK(is_admin IN (0,1)),
  token_hash TEXT NOT NULL,
  mtu INTEGER NOT NULL DEFAULT 1300 CHECK(mtu >= 1280 AND mtu <= 9000),
  routes TEXT, -- JSON array of CIDR strings, e.g. ["10.0.0.0/8", "192.168.1.0/24"]
  routes_updated_at DATETIME,
  is_lighthouse INTEGER NOT NULL DEFAULT 0 CHECK(is_lighthouse IN (0,1)),
  lighthouse_public_ip TEXT, -- Public IP for lighthouse (required if is_lighthouse=1)
  lighthouse_port INTEGER, -- UDP port for lighthouse (defaults to cluster lighthouse_port)
  is_relay INTEGER NOT NULL DEFAULT 0 CHECK(is_relay IN (0,1)),
  lighthouse_relay_updated_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(tenant_id, cluster_id, name)
);

CREATE INDEX idx_nodes_token_hash ON nodes(token_hash);
CREATE INDEX idx_nodes_cluster ON nodes(cluster_id);
CREATE INDEX idx_nodes_tenant_cluster ON nodes(tenant_id, cluster_id);
CREATE INDEX idx_nodes_lighthouse ON nodes(cluster_id, is_lighthouse) WHERE is_lighthouse = 1;
CREATE INDEX idx_nodes_relay ON nodes(cluster_id, is_relay) WHERE is_relay = 1;
```

## 3.4 config_bundles
```sql
-- +goose Up
CREATE TABLE config_bundles (
  version INTEGER NOT NULL,
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
  data BLOB NOT NULL,
  created_by TEXT REFERENCES nodes(id) ON DELETE SET NULL, -- allow node deletion while keeping bundle history
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (tenant_id, cluster_id, version)
);

CREATE INDEX idx_config_bundles_cluster ON config_bundles(tenant_id, cluster_id);
CREATE INDEX idx_config_bundles_cluster_version ON config_bundles(tenant_id, cluster_id, version DESC);
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
      middleware.go    # Auth middleware (includes replica write guard)
      handlers.go
    /auth              # Token hashing & validation logic
    /db                # SQLc generated code
      models.go
      query.sql.go
    /service           # Business logic (if needed beyond handlers)
    /lighthouse        # Control plane lighthouse management
      manager.go       # Spawns/supervises Nebula lighthouse processes per cluster
      config.go        # Generates Nebula config for lighthouse instances
      watcher.go       # Background goroutine: checks config_version every 5s, restarts lighthouses
    /ha                # High availability components
      mode.go          # Master/replica mode tracking
      replicas.go      # Replica discovery and management
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
- Logging: `go.uber.org/zap` with JSON encoder for structured logging
  - Production: JSON output to stdout
  - Development: Console-friendly output with color
  - Log levels: Debug, Info, Warn, Error, Fatal
  - Auth failures logged at WARN with full request details
- Metrics: optional `/metrics` (Prometheus) served on a separate port or under `/v1/metrics` if behind auth
- CLI: `github.com/spf13/cobra` for server and daemon bins
- TUI tables: `github.com/charmbracelet/bubbletea` + `bubbles/table` for human-readable output; wrap with `--output json` flag for machine use

### Auth Middleware Logic

1. Extract all required headers:
   - `X-Nebula-Tenant-ID`
   - `X-Nebula-Cluster-ID`
   - `X-Nebula-Node-ID`
   - `X-Nebula-Node-Token`
   - `X-Nebula-Cluster-Token`

2. Validate header presence:
   - If any header is missing, return 401 with generic "Authentication failed" message
   - Log at WARN: "Missing authentication header: <header_name>"

3. Look up node by `node_id` in database:
   - If not found, return 401
   - Log at WARN: "Node not found: node_id=<id>, tenant_id=<tid>, cluster_id=<cid>, source_ip=<ip>"

4. Verify tenant and cluster match:
   - If `node.tenant_id != header.tenant_id` OR `node.cluster_id != header.cluster_id`, return 401
   - Log at WARN: "Tenant/cluster mismatch: node_id=<id>, expected_tenant=<t1>/cluster=<c1>, provided_tenant=<t2>/cluster=<c2>, source_ip=<ip>"

5. Verify node_token:
   - Compute `HMAC-SHA256(NEBULAGC_HMAC_SECRET, header.node_token)`
   - Compare with `node.token_hash` in DB using constant-time comparison
   - If mismatch, return 401
   - Log at WARN: "Invalid node_token: node_id=<id>, provided_token=<token>, source_ip=<ip>"

6. Look up cluster and verify cluster_token:
   - Fetch cluster record by `cluster_id`
   - Compute `HMAC-SHA256(NEBULAGC_HMAC_SECRET, header.cluster_token)`
   - Compare with `cluster.cluster_token_hash` in DB
   - If mismatch, return 401
   - Log at WARN: "Invalid cluster_token: cluster_id=<id>, provided_token=<token>, source_ip=<ip>"

7. Check rate limits:
   - If IP has exceeded failed auth attempts, return 429
   - If authenticated node has exceeded request rate, return 429 with `Retry-After` header

8. Success:
   - Inject `Node` object and `Cluster` object into request context
   - Clear failed auth counter for this IP
   - Log at DEBUG: "Authenticated: node_id=<id>, tenant_id=<tid>, cluster_id=<cid>, is_admin=<true|false>, source_ip=<ip>"

### Replica Write Guard Middleware

On replica instances, add middleware after authentication that checks request method:
- If method is POST, PATCH, PUT, or DELETE:
  - Return `503 SERVICE_UNAVAILABLE`
  - Body: `{\"error\": \"This is a read-only replica. Write to master.\", \"code\": \"REPLICA_READ_ONLY\"}`
- GET requests proceed normally

### Token Generation

All tokens are generated using cryptographically secure random number generation:

```go
import "crypto/rand"

// Generate minimum 41 character token
func GenerateToken() (string, error) {
    bytes := make([]byte, 32) // 256 bits
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil // Returns 44 chars
}
```

Token requirements:
- Minimum 41 characters
- Cryptographically random (use `crypto/rand`, never `math/rand`)
- Base64 URL-safe encoding recommended
- No special meaning to token format (opaque to clients)

### Server Cobra Commands (examples)

**Server Management:**
- `nebulagc-server serve --http :8080 --db ./nebula.db --hmac-secret <secret> --master`
  - **Required:** Either `--master` or `--replicate` flag (no default, intentional choice)
  - `--master`: Run as master (accepts writes)
  - `--replicate`: Run as read-only replica (rejects writes with 503)
  - Startup fails if neither flag is provided: "Error: must specify either --master or --replicate"
  - Startup fails if both flags are provided: "Error: cannot be both master and replica"
  - `--instance-id <uuid>`: Unique ID for this control plane instance (auto-generated if not provided)
  - Optionally: `--admin-socket /var/run/nebulagc_admin.sock`
  - Required env var: `NEBULAGC_HMAC_SECRET` (if not provided via flag)
  - **Lighthouse watcher:** Automatically starts background goroutine checking cluster state every 5 seconds

**Tenant Management (CLI/Socket only):**
- `nebulagc-server tenant create --name Acme`
  - Returns: `tenant_id`
- `nebulagc-server tenant list [--output json]`

**Cluster Management (CLI/Socket only):**
- `nebulagc-server cluster create --tenant-id <tid> --name prod [--provide-lighthouse] [--lighthouse-port 4242]`
  - Returns: `cluster_id` and `cluster_token` (41+ chars)
  - Generates and prints `cluster_token` - save this securely!
  - `--provide-lighthouse`: Enable control plane as lighthouse for this cluster
  - `--lighthouse-port`: UDP port for lighthouse (default: 4242)
- `nebulagc-server cluster list --tenant-id <tid> [--output json]`

**Node Management (CLI/Socket only):**
- `nebulagc-server node create --tenant-id <tid> --cluster-id <cid> --name admin-1 --admin`
  - Omit `--admin` flag for regular nodes (defaults to false)
  - Returns: `node_id`, `node_token` (41+ chars), and `cluster_token`
  - Only time `node_token` is shown - save securely!
- `nebulagc-server node list --tenant-id <tid> --cluster-id <cid> [--output json]`

**Replica Management (CLI/Socket only):**
- `nebulagc-server replica add --address https://control2.example.com --role replica`
  - Registers a new replica in the `replicas` table
  - Master is also registered as a replica with `role=master`
- `nebulagc-server replica list [--output json]`
  - Lists all registered control plane instances
- `nebulagc-server replica remove --id <replica-id>`
  - Removes a replica from the registry

**Notes:**
- All create commands generate and display tokens exactly once
- Admin socket is disabled by default; enable with `--admin-socket` flag
- CLI and admin socket must refuse any mutating command when running in replica mode; optional preflight using `/v1/check-master` to guide the user to the master.
- Zap logging outputs JSON in production mode, console in development
- **Lighthouse watcher goroutine** starts automatically on `serve`:
  - Runs every 5 seconds (configurable via `NEBULAGC_CONFIG_CHECK_INTERVAL`)
  - For each cluster with `provide_lighthouse=true`:
    1. Query `clusters.config_version` from database
    2. Query `cluster_state.running_config_version` for this `instance_id`
    3. If `config_version > running_config_version`:
       - Read cluster PKI from database (`pki_ca_cert`, `pki_ca_key`, `pki_crl`)
       - Generate Nebula lighthouse config file
       - Restart Nebula lighthouse process for this cluster
       - Update `cluster_state.running_config_version = config_version`
       - Log at INFO: "Restarted lighthouse for cluster <id>, version <old> → <new>"
    4. If lighthouse process is dead, restart it regardless of version
  - Runs on **both master and replica instances** for N-way lighthouse redundancy

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
    BaseURLs     []string // Multiple control plane URLs for HA
    TenantID     string
    ClusterID    string
    NodeID       string
    NodeToken    string
    ClusterToken string
    HTTPClient   *http.Client
    masterURL    string // Cached master URL
}

// NewClient creates client with multiple control plane URLs for HA
func NewClient(baseURLs []string, tenantID, clusterID, nodeID, nodeToken, clusterToken string) *Client

// Internal: discovers master by querying /v1/check-master on each URL
func (c *Client) discoverMaster(ctx context.Context) error

// Internal: tries each URL until success, prefers master for writes
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, preferMaster bool) (*http.Response, error)

func (c *Client) CreateNode(ctx context.Context, name string, isAdmin bool, mtu int) (*NodeCredentials, error)
func (c *Client) RotateNodeToken(ctx context.Context, nodeID string) (string, error)
func (c *Client) DeleteNode(ctx context.Context, nodeID string) error
func (c *Client) ListNodes(ctx context.Context, page, pageSize int) ([]NodeSummary, error)
func (c *Client) RotateClusterToken(ctx context.Context) (string, error)
func (c *Client) GetLatestVersion(ctx context.Context) (int, error)
func (c *Client) DownloadBundle(ctx context.Context, current int) ([]byte, int, error)
func (c *Client) UploadBundle(ctx context.Context, data []byte) (int, error)
func (c *Client) UpdateMTU(ctx context.Context, nodeID string, mtu int) error
func (c *Client) RegisterRoutes(ctx context.Context, routes []string) error
func (c *Client) GetRoutes(ctx context.Context) ([]string, error)
func (c *Client) ListClusterRoutes(ctx context.Context) ([]NodeRoutes, error)
func (c *Client) SetLighthouse(ctx context.Context, nodeID string, enabled bool, publicIP string, port int) error
func (c *Client) SetRelay(ctx context.Context, nodeID string, enabled bool) error
func (c *Client) GetTopology(ctx context.Context) (*ClusterTopology, error)
func (c *Client) GetReplicas(ctx context.Context) ([]ReplicaInfo, error) // Discover all control plane instances
func (c *Client) CheckMaster(ctx context.Context, url string) (bool, error) // Check if specific URL is master
```

**HA Behavior:**
- Write operations (Create/Delete/Upload/Update/Set*/Rotate*) always go to master
- Read operations can use any available instance
- Automatic failover if primary instance is unreachable
- Master discovery cached and refreshed on failures

**Types:**
- `NodeCredentials`: `{NodeID, NodeToken, ClusterToken, CreatedAt}`
- `NodeSummary`: `{NodeID, Name, IsAdmin, MTU, IsLighthouse, IsRelay, Routes []string, CreatedAt, UpdatedAt}`

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

**High Availability Support:**
- Specify multiple control plane addresses in `control_plane_urls` array
- Daemon queries `/v1/check-master` on each to find master for writes
- Uses any available instance for reads (config downloads)
- Falls back to other instances if one is unavailable

```json
{
  "control_plane_urls": [
    "https://control1.example.com/v1",
    "https://control2.example.com/v1",
    "https://control3.example.com/v1"
  ],
  "clusters": [
    {
      "name": "prod-eu-west",
      "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
      "cluster_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "node_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "node_token": "bXktc2VjcmV0LW5vZGUtdG9rZW4tZm9yLWF1dGhlbnRpY2F0aW9u",
      "cluster_token": "Y2x1c3Rlci1zaGFyZWQtc2VjcmV0LWZvci1hbGwtbm9kZXMtaW4tY2x1c3Rlcg",
      "config_dir": "/etc/nebula/prod-eu-west"
    },
    {
      "name": "management-net",
      "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
      "cluster_id": "8d7e4680-8ead-22e2-91c5-11d15fe541d9",
      "node_id": "9f8f5791-9536-51f3-a2e6-22e26gf652ea",
      "node_token": "YW5vdGhlci1zZWNyZXQtbm9kZS10b2tlbi1mb3ItY2x1c3Rlci10d28",
      "cluster_token": "ZGlmZmVyZW50LWNsdXN0ZXItc2VjcmV0LWZvci1tYW5hZ2VtZW50LW5ldA",
      "config_dir": "/etc/nebula/management-net"
    }
  ]
}
```

**Notes:**
- Daemon uses `control_plane_urls` array (defined at top level) for all clusters
- For writes (node creation, config uploads), daemon queries each URL's `/v1/check-master` to find master
- For reads (config downloads, version checks), daemon can use any available instance
- Lighthouses discovered automatically via `/v1/replicas` endpoint
```

### Local Development Config
Provide a `dev_config.json` in repo root with HA support:

```json
{
  "control_plane_urls": ["http://127.0.0.1:8080/v1"],
  "clusters": [
    {
      "name": "dev-cluster",
      "tenant_id": "dev-tenant-id",
      "cluster_id": "dev-cluster-id",
      "node_id": "dev-node-id",
      "node_token": "dev-node-token",
      "cluster_token": "dev-cluster-token",
      "config_dir": "./tmp/dev-cluster"
    }
  ]
}
```

Git-ignore `tmp/` and `dev_config.json`.

### Daemon Logic

1. **Startup**: Read `/etc/nebulagc/config.json`.
2. **Discover control plane topology**:
   - Query `/v1/replicas` from first available URL in `control_plane_urls`
   - Cache all control plane instances (master + replicas)
   - For writes, identify master via `/v1/check-master` queries
3. **Spawn**: For each cluster entry:
   - Create a `Manager` routine.
   - Ensure `config_dir` exists.
   - Start a poller loop.
4. **Poller Loop** (per cluster):
   - Check `GetLatestVersion()` from any available control plane instance
   - If new version > current version:
     - `DownloadBundle()` from any instance (they all have PKI in database)
     - Unpack to `config_dir`.
     - Restart `nebula` process pointing to `config_dir/config.yml`.
   - If `nebula` process is not running, start it:
     - `nebula -config /etc/nebula/<name>/config.yml`
5. **Failover**: If primary control plane instance is unreachable, try next URL in list

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

**Master-only enforcement**
- The Cobra CLI and admin socket must refuse mutating commands unless the server is running with `--master`. Replicas return a clear error and non-zero exit status.
- Read-only listings may be allowed on replicas, but any action that changes DB state (create/update/delete/rotate) must only execute on the master.
- CLI/admin socket may internally call `/v1/check-master` or check the local mode flag to gate commands before attempting DB writes.

### Capabilities (super admin only)
- `tenant create --name ...`
- `cluster create --tenant-id ... --name ...`
- `node create --tenant-id ... --cluster-id ... --name ... --admin` (for initial admin distribution)
- `tenant list` / `cluster list` / `node list --tenant-id ... --cluster-id ...`
- Rotate Nebula PKI for a cluster by generating a new bundle in the cluster's folder.
- Manage control plane lighthouse processes:
  - When a cluster is created with `--provide-lighthouse`, the server automatically spawns a Nebula process
  - Each lighthouse process runs with its own config pointing to the cluster's PKI
  - Lighthouse processes are supervised and restarted on failure
  - Process management visible via `nebulagc-server lighthouse list [--output json]`

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

## Environment Variables
- `NEBULAGC_HMAC_SECRET` (required): Secret key for token hashing, minimum 32 bytes (base64 encoded)
- `NEBULAGC_LOG_LEVEL` (optional): Log level - debug, info, warn, error (default: info)
- `NEBULAGC_LOG_FORMAT` (optional): json or console (default: json in production)
- `NEBULAGC_ADMIN_SOCKET_PATH` (optional): Path to Unix socket for admin commands (disabled if not set)
- `NEBULAGC_INSTANCE_ID` (optional): Unique ID for this control plane instance (auto-generated UUID if not set)
- `NEBULAGC_CONFIG_CHECK_INTERVAL` (optional): Seconds between cluster state checks (default: 5)

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
