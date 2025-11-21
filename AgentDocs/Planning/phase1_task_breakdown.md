# Phase 1: HA Control Plane Core - Task Breakdown

## Overview
Phase 1 establishes the core control plane infrastructure including database, authentication, REST API, and high availability features. This phase is broken into 11 manageable tasks that build on each other.

---

## Task 00001: Project Structure and Go Modules Setup

**Objective**: Initialize the Go project structure, create go.mod files, and establish directory layout.

**Deliverables**:
- Root `go.mod` with workspace setup
- `server/go.mod` with required dependencies
- `sdk/go.mod` for client library
- `cmd/nebulagc/go.mod` for daemon
- Directory structure matching constitution standards
- `.gitignore` with appropriate entries
- Initial `Makefile` with common targets (build, test, format, lint)

**Dependencies**: None (foundational task)

**Testing**:
- `go mod verify` succeeds for all modules
- `go build ./...` compiles without errors
- Directory structure matches specification

**Required Packages**:
- `github.com/gin-gonic/gin`
- `modernc.org/sqlite`
- `github.com/pressly/goose/v3`
- `github.com/sqlc-dev/sqlc`
- `github.com/spf13/cobra`
- `github.com/charmbracelet/bubbletea`
- `go.uber.org/zap`

**Estimated Complexity**: Low
**Priority**: Critical (blocks all other tasks)

---

## Task 00002: Shared Models Package

**Objective**: Create the `models/` package with all core data structures, error types, and documentation.

**Deliverables**:
- `models/tenant.go` - Tenant struct with full documentation
- `models/cluster.go` - Cluster struct with PKI fields
- `models/node.go` - Node struct with lighthouse/relay/MTU fields
- `models/bundle.go` - ConfigBundle struct
- `models/replica.go` - Replica registry struct
- `models/errors.go` - Custom error types (ErrClusterNotFound, ErrUnauthorized, etc.)
- `models/topology.go` - ClusterTopology, NodeRoutes types
- Package-level documentation

**Documentation Requirements**:
- Every struct must have a descriptive comment
- Every field must be documented with purpose and constraints
- JSON tags for all API-facing fields
- DB tags for database-mapped fields

**Dependencies**: Task 00001

**Testing**:
- `go build ./models` succeeds
- Import models from other packages works
- All types are properly exported

**Estimated Complexity**: Low
**Priority**: Critical (required by all subsequent tasks)

---

## Task 00003: Database Migrations and SQLc Configuration

**Objective**: Create Goose migrations for all tables and configure SQLc for code generation.

**Deliverables**:
- `server/migrations/001_create_tenants.sql`
- `server/migrations/002_create_clusters.sql`
- `server/migrations/003_create_cluster_state.sql`
- `server/migrations/004_create_replicas.sql`
- `server/migrations/005_create_nodes.sql`
- `server/migrations/006_create_config_bundles.sql`
- `server/sqlc.yaml` configuration file
- `server/queries/tenants.sql` - SQLc queries for tenants
- `server/queries/clusters.sql` - SQLc queries for clusters
- `server/queries/nodes.sql` - SQLc queries for nodes
- `server/queries/bundles.sql` - SQLc queries for config bundles
- `server/queries/replicas.sql` - SQLc queries for replica registry
- Generated SQLc code in `server/internal/db/`

**Schema Requirements**:
- All tables from specification
- Proper indexes for performance
- Foreign key constraints with CASCADE
- CHECK constraints for boolean flags and value ranges
- UNIQUE constraints where specified

**Dependencies**: Task 00002 (models package)

**Testing**:
- Run migrations: `goose up` succeeds
- Reverse migrations: `goose down` succeeds
- SQLc generation: `sqlc generate` produces valid Go code
- Generated code compiles without errors

**Estimated Complexity**: Medium
**Priority**: Critical (required for all database operations)

---

## Task 00004: Authentication and Token Management

**Objective**: Implement token generation, hashing, and validation logic with security best practices.

**Deliverables**:
- `pkg/token/generator.go` - Cryptographically secure token generation
- `pkg/token/validator.go` - HMAC-SHA256 hashing and constant-time comparison
- `server/internal/auth/middleware.go` - Authentication middleware for Gin
- `server/internal/auth/context.go` - Request context helpers
- Unit tests for all authentication functions
- Documentation for security-critical code

**Security Requirements**:
- Minimum 41-character tokens
- Crypto/rand for token generation (never math/rand)
- HMAC-SHA256 with server secret
- Constant-time comparison to prevent timing attacks
- Generic error messages for all auth failures
- Detailed WARN-level logging for debugging

**Dependencies**: Task 00002 (models), Task 00003 (database)

**Testing**:
- Unit tests for token generation (length, randomness)
- Unit tests for hashing (consistency, different secrets)
- Unit tests for validation (correct tokens, wrong tokens, timing attacks)
- Middleware tests (valid auth, invalid auth, missing headers)
- Test coverage >90%

**Estimated Complexity**: Medium
**Priority**: Critical (security foundation)

---

## Task 00005: REST API Foundation (Router, Middleware, Base Handlers)

**Objective**: Set up Gin router, middleware stack, and handler structure without full implementation.

**Deliverables**:
- `server/internal/api/router.go` - Route definitions for all endpoints
- `server/internal/api/middleware.go` - Logging, recovery, rate limiting, replica write guard
- `server/internal/api/response.go` - Response helpers (success, error formatting)
- `server/internal/api/handlers/health.go` - Health check and master detection endpoints
- `server/internal/api/handlers/replica.go` - Replica discovery endpoint
- Structured logging with Zap
- Rate limiting implementation (in-memory token bucket)

**Endpoints (Implemented)**:
- `GET /v1/healthz` - Health check (no auth)
- `GET /v1/check-master` - Master detection (no auth)
- `GET /v1/replicas` - Replica discovery (no auth)

**Endpoints (Stub)**:
- All node management endpoints (return 501 Not Implemented)
- All config bundle endpoints (return 501 Not Implemented)
- All topology endpoints (return 501 Not Implemented)

**Dependencies**: Task 00004 (authentication)

**Testing**:
- Health check returns 200 OK
- Master detection returns correct status
- Authenticated requests with valid tokens pass middleware
- Invalid tokens return 401 with generic error
- Rate limiting works (429 after threshold)
- Replica write guard blocks writes on replica instances

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00006: High Availability (Master/Replica) Architecture

**Objective**: Implement master/replica mode detection, startup validation, and replica registry management.

**Deliverables**:
- `server/internal/ha/mode.go` - Mode detection (master/replica)
- `server/internal/ha/registry.go` - Replica registry CRUD operations
- `server/internal/ha/heartbeat.go` - Background heartbeat goroutine
- Startup validation (must specify --master or --replicate)
- Replica write guard middleware
- Replica pruning (remove stale entries after 5 minutes)

**Behavior**:
- Server startup requires `--master` or `--replicate` flag
- Master validates single master in replica registry
- All instances register themselves on startup
- Background goroutine updates `last_seen_at` every 30 seconds
- Master prunes stale replicas (no heartbeat for >5 minutes)
- Write operations on replicas return 503 REPLICA_READ_ONLY

**Dependencies**: Task 00003 (database), Task 00005 (API foundation)

**Testing**:
- Startup without flags fails with clear error
- Startup with both flags fails with clear error
- Master startup with multiple masters in DB fails
- Replica registration succeeds
- Heartbeat updates last_seen_at
- Stale replica pruning works correctly
- Write guard middleware blocks writes on replicas

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00007: Node Management API Handlers

**Objective**: Implement all node-related REST endpoints with full business logic.

**Deliverables**:
- `server/internal/api/handlers/node.go` - All node endpoints
- `server/internal/service/node.go` - Business logic for node operations
- Token generation and rotation
- MTU validation and updates
- Node deletion with cleanup

**Endpoints**:
- `POST /v1/tenants/{tenant_id}/clusters/{cluster_id}/nodes` - Create node
- `PATCH /v1/nodes/{node_id}/mtu` - Update MTU
- `POST /v1/nodes/{node_id}/token` - Rotate node token
- `DELETE /v1/nodes/{node_id}` - Delete node
- `GET /v1/nodes` - List cluster nodes (admin only)

**Dependencies**: Task 00004 (auth), Task 00005 (API), Task 00006 (HA)

**Testing**:
- Admin can create nodes
- Non-admin cannot create nodes (403)
- Created node returns token exactly once
- MTU validation (1280-9000 range)
- Token rotation invalidates old token
- Node deletion removes node and routes
- List nodes returns correct data (no tokens)

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00008: Config Bundle Management

**Objective**: Implement config bundle upload, download, and version management.

**Deliverables**:
- `server/internal/api/handlers/bundle.go` - Bundle endpoints
- `server/internal/service/bundle.go` - Bundle validation and storage
- `pkg/bundle/validator.go` - Bundle format and content validation
- Bundle versioning with last-write-wins strategy

**Endpoints**:
- `GET /v1/config/version` - Get latest version
- `GET /v1/config/bundle?current_version=N` - Download bundle (304 if unchanged)
- `POST /v1/config/bundle` - Upload bundle (admin only)

**Validation**:
- Maximum 10 MiB size
- Valid gzip tar format
- Required files present (config.yml, ca.crt, crl.pem, host.crt, host.key)
- Valid YAML syntax in config.yml

**Dependencies**: Task 00007 (node management)

**Testing**:
- Upload valid bundle succeeds
- Upload oversized bundle returns 413
- Upload corrupt archive returns 400
- Upload missing required files returns 400
- Download with current version returns 304
- Download with old version returns 200 with new bundle
- Concurrent uploads both succeed with incremented versions

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00009: Topology Management (Routes, Lighthouses, Relays)

**Objective**: Implement route registration, lighthouse/relay assignment, and topology queries.

**Deliverables**:
- `server/internal/api/handlers/topology.go` - Topology endpoints
- `server/internal/service/topology.go` - Business logic
- Route validation (CIDR notation)
- Lighthouse/relay status management

**Endpoints**:
- `POST /v1/routes` - Register internal routes (any node)
- `GET /v1/routes` - Get current node routes
- `GET /v1/routes/all` - List all cluster routes
- `POST /v1/nodes/{node_id}/lighthouse` - Set lighthouse status (admin)
- `POST /v1/nodes/{node_id}/relay` - Set relay status (admin)
- `GET /v1/topology` - Get cluster topology
- `POST /v1/cluster/token` - Rotate cluster token (admin)

**Dependencies**: Task 00007 (node management)

**Testing**:
- Any node can register routes
- Invalid CIDR returns 400
- Empty routes array clears routes
- Admin can set lighthouse status
- Non-admin cannot set lighthouse (403)
- Topology query returns all lighthouses and relays
- Cluster token rotation invalidates old token

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00010: Server CLI with Cobra

**Objective**: Implement command-line interface for super-admin operations with Bubble Tea output.

**Deliverables**:
- `server/cmd/nebulagc-server/main.go` - CLI entry point
- `server/cmd/nebulagc-server/cmd/serve.go` - Server start command
- `server/cmd/nebulagc-server/cmd/tenant.go` - Tenant management
- `server/cmd/nebulagc-server/cmd/cluster.go` - Cluster management
- `server/cmd/nebulagc-server/cmd/node.go` - Node management
- `server/cmd/nebulagc-server/cmd/replica.go` - Replica management
- Bubble Tea table views for list commands
- `--output json` flag for all commands

**Commands**:
- `serve --master|--replicate --http :8080 --db ./nebula.db`
- `tenant create --name <name>`
- `tenant list [--output json]`
- `cluster create --tenant-id <id> --name <name> [--provide-lighthouse]`
- `cluster list --tenant-id <id> [--output json]`
- `node create --tenant-id <id> --cluster-id <id> --name <name> [--admin]`
- `node list --tenant-id <id> --cluster-id <id> [--output json]`
- `replica add --address <url> --role <master|replica>`
- `replica list [--output json]`
- `replica remove --id <id>`

**Master-only enforcement**:
- All mutating commands check if running as master
- Replicas return error: "This is a read-only replica. Run command on master."
- Non-zero exit code on failure

**Dependencies**: Task 00009 (complete API)

**Testing**:
- All create commands generate and display tokens
- Tenant creation succeeds
- Cluster creation with lighthouse flag works
- Node creation returns credentials
- List commands show Bubble Tea tables
- JSON output is valid and parseable
- Replica commands refuse writes on replica instances

**Estimated Complexity**: Medium-High
**Priority**: High

---

## Task 00011: Lighthouse Process Management

**Objective**: Implement control plane lighthouse functionality with automatic process management.

**Deliverables**:
- `server/internal/lighthouse/manager.go` - Process lifecycle management
- `server/internal/lighthouse/config.go` - Nebula config generation
- `server/internal/lighthouse/watcher.go` - Background version checking
- `server/internal/lighthouse/supervisor.go` - Process supervision and restart
- Integration with cluster `config_version` updates

**Functionality**:
- Background goroutine runs every 5 seconds (configurable via env)
- Checks each cluster with `provide_lighthouse=true`
- Compares `config_version` vs `running_config_version` for this instance
- On version mismatch:
  - Reads PKI from database
  - Queries `/v1/replicas` for all control plane instances
  - Generates Nebula lighthouse config
  - Restarts Nebula process for this cluster
  - Updates `running_config_version` in `cluster_state`
- Runs on both master and replica instances
- Supervises lighthouse processes (restarts if crashed)

**Dependencies**: Task 00010 (complete server)

**Testing**:
- Cluster creation with lighthouse spawns process
- Config version increment triggers restart
- Process crash triggers automatic restart
- Multiple instances run lighthouses independently
- Lighthouse config includes all control plane addresses
- `cluster_state` table tracks version per instance

**Estimated Complexity**: High
**Priority**: High

---

## Phase 1 Completion Criteria

All tasks completed when:
- ✅ Server compiles and runs with `--master` or `--replicate`
- ✅ All REST endpoints functional
- ✅ Authentication and authorization working
- ✅ Database migrations applied successfully
- ✅ Cluster with lighthouse support spawns Nebula process
- ✅ Config version changes trigger lighthouse restarts
- ✅ CLI commands work for all super-admin operations
- ✅ Master/replica mode enforced correctly
- ✅ Unit tests passing with >80% coverage
- ✅ Integration tests for core workflows pass

---

## Task Dependencies Diagram

```
00001 (Project Setup)
  └─→ 00002 (Models Package)
       └─→ 00003 (Database & SQLc)
            └─→ 00004 (Authentication)
                 └─→ 00005 (API Foundation)
                      ├─→ 00006 (HA Architecture)
                      │    └─→ 00007 (Node Management)
                      │         ├─→ 00008 (Config Bundles)
                      │         └─→ 00009 (Topology)
                      │              └─→ 00010 (Server CLI)
                      └─→ 00011 (Lighthouse Management)
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

After Phase 1 completion, proceed to:
- **Phase 2**: Go SDK and nebulagc daemon implementation
- **Phase 3**: Ops hardening (rate limiting, monitoring, deployment docs)
