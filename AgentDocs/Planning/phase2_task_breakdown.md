# Phase 2: Node-Facing SDK and Daemon - Task Breakdown

## Overview
Phase 2 builds the client-side components: a Go SDK for programmatic access and the `nebulagc` daemon for automated multi-cluster management with polling and process supervision.

---

## Task 00012: Go Client SDK Foundation

**Objective**: Create the core SDK structure with authentication, request handling, and HA support.

**Deliverables**:
- `sdk/client.go` - Main Client struct with HA support
- `sdk/config.go` - Client configuration and initialization
- `sdk/auth.go` - Header-based authentication
- `sdk/errors.go` - SDK-specific error types
- `sdk/types.go` - Request/response types
- `sdk/transport.go` - HTTP transport with retry logic
- Master discovery implementation
- Failover logic for multiple control plane URLs

**Core Features**:
- Multiple `BaseURLs` for HA
- Master discovery via `/v1/check-master`
- Automatic failover on connection errors
- Cached master URL with refresh on failure
- Request retry logic (configurable)
- Proper header injection for all requests

**Client Interface**:
```go
type Client struct {
    BaseURLs     []string
    TenantID     string
    ClusterID    string
    NodeID       string
    NodeToken    string
    ClusterToken string
    HTTPClient   *http.Client
    masterURL    string // cached
}

func NewClient(config ClientConfig) (*Client, error)
func (c *Client) DiscoverMaster(ctx context.Context) error
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, preferMaster bool) (*http.Response, error)
```

**Dependencies**: Task 00002 (models package - for shared types)

**Testing**:
- Client creation with valid config
- Master discovery finds master
- Failover to next URL on connection failure
- Authentication headers sent correctly
- Error handling for network failures

**Estimated Complexity**: Medium
**Priority**: Critical (blocks all SDK endpoints)

---

## Task 00013: SDK Node Management Methods

**Objective**: Implement SDK methods for node lifecycle operations.

**Deliverables**:
- Node creation
- Node deletion
- Node listing
- MTU updates
- Token rotation

**Methods**:
```go
func (c *Client) CreateNode(ctx context.Context, name string, isAdmin bool, mtu int) (*NodeCredentials, error)
func (c *Client) DeleteNode(ctx context.Context, nodeID string) error
func (c *Client) ListNodes(ctx context.Context, page, pageSize int) ([]NodeSummary, error)
func (c *Client) UpdateMTU(ctx context.Context, nodeID string, mtu int) error
func (c *Client) RotateNodeToken(ctx context.Context, nodeID string) (string, error)
```

**Dependencies**: Task 00012 (SDK foundation)

**Testing**:
- Create node returns credentials
- Delete node succeeds
- List nodes returns correct data
- MTU update validates range
- Token rotation returns new token

**Estimated Complexity**: Low
**Priority**: High

---

## Task 00014: SDK Config Bundle Methods

**Objective**: Implement SDK methods for config bundle operations.

**Deliverables**:
- Version checking
- Bundle download with 304 support
- Bundle upload

**Methods**:
```go
func (c *Client) GetLatestVersion(ctx context.Context) (int, error)
func (c *Client) DownloadBundle(ctx context.Context, currentVersion int) (data []byte, newVersion int, err error)
func (c *Client) UploadBundle(ctx context.Context, data []byte) (int, error)
```

**304 Handling**:
- `DownloadBundle` returns `(nil, currentVersion, nil)` on 304 Not Modified
- Caller can distinguish "no update" from error

**Dependencies**: Task 00012 (SDK foundation)

**Testing**:
- Version check returns correct version
- Download with current version returns 304 (no data)
- Download with old version returns new bundle
- Upload succeeds and returns new version

**Estimated Complexity**: Low
**Priority**: High

---

## Task 00015: SDK Topology Methods

**Objective**: Implement SDK methods for routes, lighthouses, relays, and cluster topology.

**Deliverables**:
- Route registration and retrieval
- Lighthouse/relay status management
- Topology queries
- Cluster token rotation

**Methods**:
```go
func (c *Client) RegisterRoutes(ctx context.Context, routes []string) error
func (c *Client) GetRoutes(ctx context.Context) ([]string, error)
func (c *Client) ListClusterRoutes(ctx context.Context) ([]NodeRoutes, error)
func (c *Client) SetLighthouse(ctx context.Context, nodeID string, enabled bool, publicIP string, port int) error
func (c *Client) SetRelay(ctx context.Context, nodeID string, enabled bool) error
func (c *Client) GetTopology(ctx context.Context) (*ClusterTopology, error)
func (c *Client) RotateClusterToken(ctx context.Context) (string, error)
```

**Dependencies**: Task 00012 (SDK foundation)

**Testing**:
- Route registration works
- Route retrieval returns correct routes
- Lighthouse assignment succeeds
- Relay assignment succeeds
- Topology query returns all lighthouses and relays
- Cluster token rotation returns new token

**Estimated Complexity**: Low
**Priority**: High

---

## Task 00016: SDK Replica Discovery Methods

**Objective**: Implement SDK methods for HA replica discovery and master detection.

**Deliverables**:
- Replica discovery
- Master detection
- URL health checking

**Methods**:
```go
func (c *Client) GetReplicas(ctx context.Context) ([]ReplicaInfo, error)
func (c *Client) CheckMaster(ctx context.Context, url string) (bool, error)
func (c *Client) RefreshControlPlaneList(ctx context.Context) error
```

**Dependencies**: Task 00012 (SDK foundation)

**Testing**:
- Replica discovery returns all instances
- Master detection identifies master correctly
- Refresh updates cached instance list

**Estimated Complexity**: Low
**Priority**: Medium

---

## Task 00017: Daemon Configuration and Initialization

**Objective**: Create the daemon configuration file format and initialization logic.

**Deliverables**:
- `cmd/nebulagc/daemon/config.go` - Config file parsing
- `cmd/nebulagc/daemon/init.go` - Daemon initialization
- `/etc/nebulagc/config.json` format specification
- Development config support (`dev_config.json`)
- Config validation (required fields, valid UUIDs, etc.)

**Config Format**:
```json
{
  "control_plane_urls": ["https://control1.example.com/v1"],
  "clusters": [
    {
      "name": "prod-cluster",
      "tenant_id": "uuid",
      "cluster_id": "uuid",
      "node_id": "uuid",
      "node_token": "secret",
      "cluster_token": "secret",
      "config_dir": "/etc/nebula/prod-cluster"
    }
  ]
}
```

**Validation**:
- Required fields present
- Valid UUIDs
- Token minimum length (41 chars)
- Valid URLs for control plane
- Config directory paths are valid

**Dependencies**: Task 00016 (complete SDK)

**Testing**:
- Valid config parses correctly
- Missing required fields return error
- Invalid UUIDs return error
- Short tokens return error
- Config directory validation works

**Estimated Complexity**: Low
**Priority**: High

---

## Task 00018: Daemon Multi-Cluster Manager

**Objective**: Implement the core daemon logic for managing multiple Nebula instances.

**Deliverables**:
- `cmd/nebulagc/daemon/manager.go` - Multi-cluster management
- `cmd/nebulagc/daemon/cluster.go` - Per-cluster manager
- Control plane discovery and master detection
- Cluster initialization and validation
- Graceful shutdown handling

**Functionality**:
- Read config file
- Discover all control plane instances via `/v1/replicas`
- Identify master via `/v1/check-master`
- Spawn manager goroutine per cluster
- Coordinate startup and shutdown
- Handle signals (SIGTERM, SIGINT)

**Dependencies**: Task 00017 (daemon config)

**Testing**:
- Daemon starts with valid config
- Multiple clusters managed concurrently
- Control plane discovery works
- Master identification succeeds
- Graceful shutdown stops all clusters

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00019: Config Poller and Bundle Management

**Objective**: Implement the polling logic that checks for config updates and downloads bundles.

**Deliverables**:
- `cmd/nebulagc/daemon/poller.go` - Version polling loop
- `cmd/nebulagc/daemon/bundle.go` - Bundle download and extraction
- 5-second polling interval (configurable)
- Bundle unpacking to config directory
- Atomic file replacement to prevent corruption

**Polling Logic**:
```go
// Every 5 seconds:
1. Call SDK GetLatestVersion()
2. Compare with current version
3. If new version available:
   - Call SDK DownloadBundle()
   - Validate bundle (tar.gz format, required files)
   - Extract to temporary directory
   - Atomically replace config directory
   - Trigger Nebula process restart
4. Update current version tracking
```

**Safety**:
- Atomic file replacement (rename, not overwrite)
- Validation before extraction
- Rollback on extraction failure
- Lock file to prevent concurrent modifications

**Dependencies**: Task 00018 (daemon manager)

**Testing**:
- Polling detects new versions
- Bundle download succeeds
- Extraction creates all required files
- Atomic replacement works
- Rollback on corruption

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00020: Nebula Process Supervision

**Objective**: Implement process supervision for Nebula instances with restart logic.

**Deliverables**:
- `cmd/nebulagc/daemon/supervisor.go` - Process lifecycle management
- `cmd/nebulagc/daemon/process.go` - Process wrapper and monitoring
- Process startup and shutdown
- Crash detection and restart
- Log capture (stdout/stderr)
- Graceful stop with SIGTERM

**Supervision Features**:
- Start Nebula with correct config path
- Monitor process health
- Capture and log output (structured logging)
- Restart on crash (with backoff)
- Restart on config update
- Graceful shutdown on daemon exit

**Process Command**:
```bash
nebula -config /etc/nebula/<cluster-name>/config.yml
```

**Dependencies**: Task 00019 (poller and bundle management)

**Testing**:
- Nebula process starts correctly
- Process crash triggers restart
- Config update triggers restart
- Logs captured and forwarded
- Graceful stop works
- Backoff prevents restart loops

**Estimated Complexity**: Medium-High
**Priority**: High

---

## Task 00021: Daemon Cobra CLI

**Objective**: Create the command-line interface for the daemon with status monitoring.

**Deliverables**:
- `cmd/nebulagc/main.go` - CLI entry point
- `cmd/nebulagc/cmd/daemon.go` - Daemon mode command
- `cmd/nebulagc/cmd/status.go` - Status monitoring
- `cmd/nebulagc/cmd/cluster.go` - Cluster management commands (via SDK)
- `cmd/nebulagc/cmd/node.go` - Node management commands (via SDK)
- Bubble Tea UI for status views
- `--output json` flag support

**Commands**:
- `nebulagc daemon --config /etc/nebulagc/config.json`
  - Starts daemon in foreground
  - Manages all configured clusters
  - Logs to stdout (structured JSON)
- `nebulagc status [--output json]`
  - Shows status of all clusters
  - Nebula process state (running/stopped)
  - Current config version
  - Last update time
- `nebulagc cluster list [--output json]`
  - Lists configured clusters
- `nebulagc node create --cluster <name> --name <node-name> [--admin]`
  - Creates node via SDK (must be admin)
  - Shows credentials
- `nebulagc node list --cluster <name> [--output json]`
  - Lists nodes in cluster (admin only)

**Dependencies**: Task 00020 (complete daemon)

**Testing**:
- Daemon starts and runs
- Status command shows correct state
- Cluster list works
- Node creation via CLI succeeds (if admin)
- JSON output is valid
- Bubble Tea UI renders correctly

**Estimated Complexity**: Medium
**Priority**: Medium

---

## Task 00022: HA Failover and Resilience

**Objective**: Ensure the daemon handles control plane failures gracefully with automatic failover.

**Deliverables**:
- Failover logic in SDK
- Retry mechanisms with backoff
- Health checking for control plane instances
- Cached replica list with periodic refresh
- Degraded mode handling (read-only when master unreachable)

**Failover Behavior**:
- Write operations fail over to master only
- Read operations try any available instance
- Connection failures trigger immediate failover
- Periodic health checks (every 60 seconds)
- Stale replica list refresh on failures
- Exponential backoff for repeated failures

**Degraded Mode**:
- If master unreachable, daemon continues with existing config
- Logs WARNING about degraded state
- Continues polling from replicas for reads
- Defers writes until master available
- Clear status indication in `nebulagc status`

**Dependencies**: Task 00021 (daemon CLI)

**Testing**:
- Master failure triggers failover
- Replica failure falls back to others
- Write fails gracefully when master down
- Reads continue from replicas
- Health checks detect failures
- Replica list refresh works
- Status shows degraded mode

**Estimated Complexity**: Medium-High
**Priority**: High

---

## Phase 2 Completion Criteria

All tasks completed when:
- ✅ SDK compiles and all methods functional
- ✅ SDK handles HA failover correctly
- ✅ Daemon reads config and discovers control plane
- ✅ Daemon manages multiple clusters concurrently
- ✅ Polling detects config updates and downloads bundles
- ✅ Nebula processes supervised and restarted on crashes
- ✅ CLI commands work for status and management
- ✅ Failover works when control plane instances fail
- ✅ Unit tests passing with >80% coverage
- ✅ Integration tests for SDK and daemon pass
- ✅ End-to-end test: daemon enrolls node, downloads config, starts Nebula

---

## Task Dependencies Diagram

```
Phase 1 Complete (00011)
  └─→ 00012 (SDK Foundation)
       ├─→ 00013 (SDK Node Methods)
       ├─→ 00014 (SDK Bundle Methods)
       ├─→ 00015 (SDK Topology Methods)
       └─→ 00016 (SDK Replica Discovery)
            └─→ 00017 (Daemon Config)
                 └─→ 00018 (Daemon Manager)
                      └─→ 00019 (Config Poller)
                           └─→ 00020 (Process Supervision)
                                └─→ 00021 (Daemon CLI)
                                     └─→ 00022 (HA Failover)
```

---

## Validation Process

For each task:
1. Move task file from `ToDo/` to `InProgress/` with next sequential number
2. Implement according to constitution standards
3. Document all functions, structs, and fields
4. Write unit tests (>80% coverage target)
5. Ensure all tests pass
6. Create git commit referencing task number
7. Move task file to `Done/` keeping same number
8. Update task file with completion date

---

## Next Steps

After Phase 2 completion, proceed to:
- **Phase 3**: Ops hardening, deployment docs, and production tooling
