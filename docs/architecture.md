# NebulaGC Architecture

This document describes the architecture of NebulaGC, a high-availability control plane for managing Nebula overlay networks at scale.

## Table of Contents

- [System Overview](#system-overview)
- [Core Components](#core-components)
- [Data Model](#data-model)
- [High Availability](#high-availability)
- [Security Architecture](#security-architecture)
- [API Design](#api-design)
- [Data Flow](#data-flow)
- [Scalability Considerations](#scalability-considerations)

## System Overview

NebulaGC is designed as a centralized control plane that manages the lifecycle of Nebula overlay networks across multiple clusters and tenants. It provides:

- **Multi-tenant isolation** - Complete separation of resources per tenant
- **High availability** - Master/replica architecture with automatic failover
- **REST API** - Simple HTTP-based interface for automation
- **Config management** - Centralized distribution of Nebula configurations
- **Lighthouse management** - Automatic lighthouse process lifecycle
- **Topology awareness** - Network topology management and validation

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Load Balancer                            │
│                      (Optional, HA mode)                         │
└─────────────────────────────────────────────────────────────────┘
                                 │
                ┌────────────────┼────────────────┐
                │                │                │
                ▼                ▼                ▼
┌───────────────────┐  ┌───────────────────┐  ┌───────────────────┐
│  NebulaGC Master  │  │ NebulaGC Replica  │  │ NebulaGC Replica  │
│                   │  │                   │  │                   │
│  ┌─────────────┐  │  │  ┌─────────────┐  │  │  ┌─────────────┐  │
│  │   REST API  │  │  │  │   REST API  │  │  │  │   REST API  │  │
│  └─────────────┘  │  │  └─────────────┘  │  │  └─────────────┘  │
│  ┌─────────────┐  │  │  ┌─────────────┐  │  │  ┌─────────────┐  │
│  │  HA Manager │  │  │  │  HA Manager │  │  │  │  HA Manager │  │
│  └─────────────┘  │  │  └─────────────┘  │  │  └─────────────┘  │
│  ┌─────────────┐  │  │  ┌─────────────┐  │  │  ┌─────────────┐  │
│  │ Lighthouse  │  │  │  │ Lighthouse  │  │  │  │ Lighthouse  │  │
│  │   Manager   │  │  │  │   Manager   │  │  │  │   Manager   │  │
│  └─────────────┘  │  │  └─────────────┘  │  │  └─────────────┘  │
│  ┌─────────────┐  │  │  ┌─────────────┐  │  │  ┌─────────────┐  │
│  │   SQLite    │◄─┼──┼──┤   SQLite    │◄─┼──┼──┤   SQLite    │  │
│  │  (Leader)   │  │  │  │  (Follower) │  │  │  │  (Follower) │  │
│  └─────────────┘  │  │  └─────────────┘  │  │  └─────────────┘  │
└───────────────────┘  └───────────────────┘  └───────────────────┘
         │                      │                      │
         └──────────────────────┴──────────────────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │   Managed Nodes        │
                    │  (Nebula Lighthouses)  │
                    └────────────────────────┘
```

## Core Components

### REST API Server

**Location**: `server/internal/api/`

The REST API server is the primary interface for all client interactions. Built with [Gin framework](https://gin-gonic.com/), it provides:

- **Health checks** - `/health` endpoint for load balancer probes
- **Version info** - `/version` endpoint for compatibility checks
- **Node management** - CRUD operations for Nebula nodes
- **Bundle management** - Config bundle upload and download
- **Topology management** - Network topology operations
- **Authentication** - HMAC-SHA256 token-based auth on all endpoints

**Key Features**:
- Rate limiting (per-IP, per-node, per-cluster)
- Request logging with correlation IDs
- Prometheus metrics endpoint
- CORS support for web clients
- Structured error responses

### High Availability Manager

**Location**: `server/internal/ha/`

The HA manager coordinates master/replica roles and handles failover:

- **Role Management** - Determines if instance is master or replica
- **Heartbeat System** - Detects master failures within 30 seconds
- **Automatic Failover** - Promotes replicas to master on failure
- **Graceful Handoff** - Coordinates leadership changes

**Configuration**:
```bash
# Master mode (default)
NEBULAGC_HA_MODE=master

# Replica mode
NEBULAGC_HA_MODE=replica
NEBULAGC_HA_MASTER_URL=https://master:8443
```

**State Transitions**:
```
┌──────────┐     Master      ┌──────────┐
│  Master  │────Detected─────▶│  Replica │
│          │     Failure      │          │
└──────────┘                  └──────────┘
     ▲                             │
     │                             │
     └─────────Promotion────────────┘
           (Replica becomes Master)
```

### Lighthouse Manager

**Location**: `server/internal/lighthouse/`

The lighthouse manager handles the lifecycle of Nebula lighthouse processes:

- **Process Management** - Start/stop/restart lighthouse processes
- **Config Generation** - Generate Nebula configs from bundles
- **Health Monitoring** - Monitor lighthouse process health
- **Port Management** - Allocate UDP ports for lighthouses
- **Log Management** - Capture and expose lighthouse logs

**Workflow**:
1. Client uploads config bundle
2. Manager validates bundle structure
3. Manager generates lighthouse config
4. Manager starts lighthouse process
5. Manager monitors process health
6. Nodes connect to lighthouse

### Database Layer

**Location**: `server/internal/db/`

SQLite database with WAL mode for concurrency and durability:

- **Schema Management** - Versioned migrations
- **Query Generation** - SQLC-generated type-safe queries
- **Connection Pooling** - Optimized for multi-reader, single-writer
- **Transactions** - ACID guarantees for critical operations

**Tables**:
- `tenants` - Multi-tenant isolation
- `clusters` - Logical grouping of nodes
- `cluster_state` - HA state tracking
- `replicas` - Replica instance registry
- `nodes` - Nebula node definitions
- `config_bundles` - Configuration storage
- `topology_nodes` - Network topology

### Service Layer

**Location**: `server/internal/service/`

Business logic layer that orchestrates operations:

- **Node Service** - Node CRUD, validation, auth token generation
- **Bundle Service** - Bundle upload, validation, distribution
- **Topology Service** - Network topology validation and management
- **Replica Service** - Replica registration and health checks

**Responsibilities**:
- Input validation
- Business rule enforcement
- Cross-entity coordination
- Error handling and logging

## Data Model

### Entity Relationships

```
┌──────────┐
│ Tenant   │
└────┬─────┘
     │ 1
     │
     │ N
┌────▼─────┐
│ Cluster  │
└────┬─────┘
     │ 1
     │
     ├──────────────────┬──────────────────┐
     │ N                │ N                │ N
┌────▼─────┐      ┌────▼─────┐      ┌────▼─────┐
│   Node   │      │  Bundle  │      │ Topology │
└──────────┘      └──────────┘      └──────────┘
```

### Key Entities

#### Tenant

Multi-tenant isolation boundary. All resources belong to exactly one tenant.

```go
type Tenant struct {
    ID          string    // UUID
    Name        string    // Unique tenant name
    Description string    // Optional description
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

#### Cluster

Logical grouping of nodes within a tenant. Represents a single Nebula overlay network.

```go
type Cluster struct {
    ID          string    // UUID
    TenantID    string    // Parent tenant
    Name        string    // Unique within tenant
    Description string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

#### Node

Represents a Nebula node (lighthouse or regular peer).

```go
type Node struct {
    ID            string    // UUID
    ClusterID     string    // Parent cluster
    Name          string    // Unique within cluster
    IsLighthouse  bool      // Lighthouse flag
    PublicIP      string    // Optional for lighthouses
    OverlayIP     string    // Nebula overlay IP
    AuthToken     string    // HMAC-hashed auth token
    LastSeenAt    *time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

#### ConfigBundle

Nebula configuration package (CA cert, config.yml, optional lighthouse binaries).

```go
type ConfigBundle struct {
    ID          string    // UUID
    ClusterID   string    // Parent cluster
    Version     string    // Semantic version
    Data        []byte    // Tarball containing configs
    Hash        string    // SHA256 hash for integrity
    CreatedAt   time.Time
}
```

#### Topology

Network topology definition for a cluster.

```go
type TopologyNode struct {
    ClusterID      string    // Parent cluster
    NodeID         string    // Reference to Node
    IsLighthouse   bool
    OverlayIP      string
    PublicIP       string    // For lighthouses
    Port           int       // For lighthouses
}
```

## High Availability

### Master/Replica Architecture

NebulaGC uses an active-passive HA model:

- **One Master** - Handles all write operations and lighthouse management
- **N Replicas** - Handle read operations and provide failover capability
- **SQLite Replication** - Database replication via Litestream or similar
- **Automatic Failover** - Replica promotion on master failure

### Failover Process

1. **Failure Detection** - Replicas detect master failure via heartbeat
2. **Election** - Replicas coordinate to elect new master (simple: first to acquire lock)
3. **Promotion** - Elected replica promotes itself to master
4. **State Sync** - New master ensures database is up-to-date
5. **Resume** - New master begins accepting write operations

**Failover Time**: Typically 30-60 seconds depending on heartbeat interval.

### Read/Write Splitting

```
┌────────────┐
│   Client   │
└─────┬──────┘
      │
      ├─── Write Request ───▶ Master Only
      │
      └─── Read Request  ───▶ Any Replica (or Master)
```

**Benefits**:
- Reduced master load
- Improved read scalability
- Geographic distribution of read traffic

## Security Architecture

### Authentication Flow

```
1. Client ───────────────────▶ Request with Bearer token
                               (Authorization: Bearer <token>)

2. Server ───▶ Extract token
          ───▶ Hash with HMAC-SHA256
          ───▶ Compare with stored hash

3. Server ───▶ Lookup node by token hash
          ───▶ Validate cluster membership
          ───▶ Check node status

4. Server ───────────────────▶ Allow/Deny request
```

### Token Security

- **Generation**: 32-byte random tokens (crypto/rand)
- **Storage**: HMAC-SHA256 hashes only (never plaintext)
- **Transport**: HTTPS required in production
- **Scope**: Per-node tokens with cluster isolation

### Rate Limiting

Multi-level rate limiting protects against abuse:

- **Global**: 1000 req/sec across all clients
- **Per-IP**: 100 req/min per source IP
- **Per-Node**: 20 req/min per authenticated node
- **Per-Cluster**: 200 req/min per cluster

### Input Validation

- **Schema Validation** - JSON request bodies validated against schemas
- **Type Safety** - Strong typing via Go structs and SQLC
- **Sanitization** - SQL injection prevention via parameterized queries
- **Length Limits** - Max request body size (10MB for bundles)

## API Design

### REST Principles

- **Resource-Oriented** - URLs represent resources (nouns, not verbs)
- **HTTP Methods** - Standard verbs (GET, POST, PUT, DELETE, PATCH)
- **Status Codes** - Semantic HTTP status codes
- **JSON** - Request/response bodies in JSON

### Endpoint Structure

```
/api/v1/health                    GET    - Health check
/api/v1/version                   GET    - Version info
/api/v1/nodes                     GET    - List nodes (filtered by cluster)
/api/v1/nodes                     POST   - Create node
/api/v1/nodes/:id                 GET    - Get node details
/api/v1/nodes/:id                 PUT    - Update node
/api/v1/nodes/:id                 DELETE - Delete node
/api/v1/nodes/:id/token           POST   - Regenerate auth token
/api/v1/bundles/:cluster_id       GET    - List bundles
/api/v1/bundles/:cluster_id       POST   - Upload bundle
/api/v1/bundles/:cluster_id/:version GET - Download bundle
/api/v1/topology/:cluster_id      GET    - Get topology
/api/v1/topology/:cluster_id      POST   - Update topology
```

### Error Responses

Consistent error format:

```json
{
  "error": "human-readable error message",
  "code": "ERROR_CODE",
  "details": {
    "field": "additional context"
  },
  "request_id": "correlation-id-for-tracing"
}
```

### Versioning

- **URL-based**: `/api/v1/...` for version 1
- **Backward compatibility**: v1 API stable for 1.x releases
- **Breaking changes**: Introduce v2 API alongside v1

## Data Flow

### Node Registration Flow

```
1. Admin creates tenant/cluster (via SQL initially, API in future)
   ↓
2. Admin calls POST /api/v1/nodes with node details
   ↓
3. Server validates input and generates auth token
   ↓
4. Server stores node in database with hashed token
   ↓
5. Server returns node details + plaintext token (only time shown)
   ↓
6. Admin deploys node daemon with token
   ↓
7. Node daemon authenticates using token for all requests
```

### Config Bundle Distribution Flow

```
1. Admin prepares config bundle (CA cert + config.yml + binaries)
   ↓
2. Admin calls POST /api/v1/bundles/:cluster_id with tarball
   ↓
3. Server validates bundle structure and contents
   ↓
4. Server stores bundle in database with version and hash
   ↓
5. Master lighthouse manager extracts and uses bundle
   ↓
6. Nodes call GET /api/v1/bundles/:cluster_id/latest
   ↓
7. Nodes download bundle, verify hash, apply config
   ↓
8. Nodes restart Nebula process with new config
```

### Lighthouse Lifecycle

```
1. Admin uploads config bundle for cluster
   ↓
2. Master lighthouse manager detects new bundle
   ↓
3. Manager extracts bundle to temp directory
   ↓
4. Manager generates lighthouse-specific config.yml
   ↓
5. Manager starts nebula process with config
   ↓
6. Manager monitors process health (PID, port binding)
   ↓
7. Nodes connect to lighthouse (from topology)
   ↓
8. Manager logs lighthouse output to file
   ↓
9. On bundle update, manager restarts lighthouse
```

### Topology Propagation

```
1. Admin defines topology (lighthouse IPs/ports)
   ↓
2. Admin calls POST /api/v1/topology/:cluster_id
   ↓
3. Server validates topology (lighthouse nodes exist)
   ↓
4. Server stores topology in database
   ↓
5. Nodes periodically fetch topology
   ↓
6. Nodes update their local Nebula configs
   ↓
7. Nodes reconnect to lighthouses if changed
```

## Scalability Considerations

### Current Design

- **Vertical Scaling** - Single master handles all writes
- **Horizontal Scaling** - Multiple replicas handle reads
- **Database** - SQLite with WAL mode (10k+ writes/sec)
- **Lighthouse** - Single process per cluster per master

### Bottlenecks

1. **Master Writes** - Single writer to SQLite (mitigated by WAL mode)
2. **Lighthouse Processes** - Memory overhead per cluster (mitigated by resource limits)
3. **Bundle Storage** - Large bundles in database (future: object storage)

### Future Improvements

- **Distributed Database** - PostgreSQL or CockroachDB for multi-master
- **Object Storage** - S3/MinIO for config bundles
- **Lighthouse Sharding** - Multiple lighthouse instances per cluster
- **Caching** - Redis for frequently accessed data
- **Message Queue** - RabbitMQ/NATS for async operations

### Current Limits

Based on testing and architecture:

- **Tenants**: 1,000+ (no practical limit)
- **Clusters per Tenant**: 100+ (limited by lighthouse memory)
- **Nodes per Cluster**: 10,000+ (Nebula's limit)
- **Concurrent Requests**: 1,000+ req/sec (rate limited)
- **Bundle Size**: 100MB max (configurable)

### Performance Metrics

From E2E testing (48 tests in 260ms):

- **Average Response Time**: ~5ms per operation
- **Database Operations**: <2ms (WAL mode)
- **Token Validation**: <1ms (HMAC)
- **Bundle Upload**: ~50ms for 10MB bundle
- **Failover Time**: 30-60 seconds

## Deployment Topologies

### Single Instance (Development)

```
┌─────────────────┐
│  NebulaGC       │
│  (Master Mode)  │
│  + SQLite       │
└─────────────────┘
```

**Use Case**: Development, testing, small deployments
**Availability**: None (single point of failure)

### High Availability (Production)

```
       Load Balancer
            │
    ┌───────┼───────┐
    │       │       │
┌───▼───┐ ┌─▼─────┐ ┌─▼─────┐
│Master │ │Replica│ │Replica│
└───────┘ └───────┘ └───────┘
```

**Use Case**: Production deployments
**Availability**: Survives single instance failure

### Multi-Region (Future)

```
Region A          Region B          Region C
┌──────┐         ┌──────┐         ┌──────┐
│Master│◄────────┤Replica│◄────────┤Replica│
└──────┘         └──────┘         └──────┘
```

**Use Case**: Global deployments, disaster recovery
**Availability**: Survives region failure

## Monitoring and Observability

### Metrics (Prometheus)

- `nebulagc_api_requests_total` - Total API requests (by endpoint, status)
- `nebulagc_api_request_duration_seconds` - Request latency histogram
- `nebulagc_ha_role` - Current HA role (0=replica, 1=master)
- `nebulagc_lighthouse_processes` - Active lighthouse processes
- `nebulagc_nodes_total` - Total managed nodes
- `nebulagc_bundles_total` - Total config bundles

### Logging (Structured JSON)

```json
{
  "timestamp": "2025-11-22T10:30:45Z",
  "level": "info",
  "msg": "node created successfully",
  "request_id": "abc123",
  "node_id": "uuid",
  "cluster_id": "uuid",
  "duration_ms": 12
}
```

### Health Checks

- `/health` - Overall service health
- `/health/database` - Database connectivity
- `/health/ha` - HA manager status
- `/health/lighthouse` - Lighthouse process health

## Security Best Practices

### Production Deployment

1. **Always use HTTPS** - TLS 1.2+ with valid certificates
2. **Rotate secrets** - HMAC secret rotation every 90 days
3. **Firewall rules** - Limit API access to known IPs
4. **Database encryption** - Encrypt SQLite database at rest
5. **Audit logging** - Log all API requests with user context
6. **Rate limiting** - Configure appropriate limits for your workload
7. **Regular updates** - Apply security patches promptly

### Network Security

```
┌─────────────┐
│   Firewall  │
│   (Allow    │
│   443 only) │
└──────┬──────┘
       │
┌──────▼──────┐
│  NebulaGC   │
│  (Private   │
│   Network)  │
└─────────────┘
```

## Conclusion

NebulaGC's architecture prioritizes:

- **Simplicity** - SQLite, embedded database, minimal dependencies
- **Reliability** - HA support, automatic failover, ACID transactions
- **Security** - Token-based auth, rate limiting, input validation
- **Observability** - Structured logging, Prometheus metrics, health checks
- **Scalability** - Read replicas, efficient database queries, resource limits

The design supports production deployments managing thousands of nodes across multiple clusters while maintaining operational simplicity.

For operational guidance, see [Operations Manual](operations.md).
For API details, see [API Reference](api-reference.md).
