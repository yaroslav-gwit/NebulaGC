# NebulaGC - Claude Code Project Guide

## ⚠️ MANDATORY: AI Agent Session Initialization

**At the start of EVERY new session, AI agents MUST:**

1. **Read Core Documentation** (in this order):
   - [claude.md](claude.md) - This file (project overview)
   - [AgentDocs/constitution.md](AgentDocs/constitution.md) - Project constitution and coding standards
   - [AgentDocs/ToDo/nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md) - Complete technical specification

2. **Review Current Work State**:
   - Check `AgentDocs/InProgress/` for active tasks
   - Check `AgentDocs/Done/` for completed work (identify highest task number)
   - Determine next sequential task number for new work

3. **Understand Task Numbering System**:
   - All tasks in `InProgress/` and `Done/` MUST use format: `XXXXX_<task_name>.md`
   - Example: `00001_implement_cluster_service.md`, `00002_add_authentication.md`
   - Numbers are sequential and zero-padded to 5 digits
   - This enables easy rollback and change tracking

4. **Confirm Standards**:
   - All functions must have documentation comments
   - All structs and fields must be documented
   - Models must be in separate `models/` package for reusability
   - Follow all guidelines in [AgentDocs/constitution.md](AgentDocs/constitution.md)

**Failure to follow these initialization steps may result in incorrect implementation or violation of project standards.**

---

## Project Overview

**NebulaGC (Nebula Ground Control)** is a Go-based control plane for managing Nebula overlay networks. The project provides a lightweight, multi-tenant solution for configuration management, certificate distribution, and node enrollment in Nebula mesh networks.

**Current Status**: Specification/Planning Phase (No implementation code yet)

**Purpose**:
- Versioned configuration management for Nebula mesh networks
- Multi-tenant cluster management
- Certificate distribution and PKI management
- Node enrollment and authentication
- High availability architecture with master/replica support

## Repository Structure

```
NebulaGC/
├── AgentDocs/                               # Development workflow tracking
│   ├── constitution.md                      # Project coding standards (READ FIRST!)
│   ├── Done/                                # Completed tasks (numbered: 00001_*.md)
│   ├── InProgress/                          # Current work (numbered: 00001_*.md)
│   └── ToDo/                                # Planned tasks (unnumbered specs)
│       └── nebula_control_plane_spec.md    # Complete technical specification
├── README.md                                # Project overview and quick start
└── claude.md                                # This file (AI agent guide)
```

## Planned Implementation Structure

Based on the specification, the project will follow this structure:

```
NebulaGC/
├── server/                          # Control plane server
│   ├── cmd/
│   │   └── nebulagc-server/         # Main server binary
│   ├── internal/
│   │   ├── api/                     # HTTP handlers and routing
│   │   ├── auth/                    # Token hashing and validation
│   │   ├── db/                      # SQLc generated code
│   │   ├── service/                 # Business logic
│   │   ├── lighthouse/              # Lighthouse process management
│   │   └── ha/                      # High availability
│   ├── migrations/                  # Database migrations
│   └── sqlc.yaml                    # SQLc configuration
├── cmd/
│   └── nebulagc/                    # Client daemon/CLI
├── sdk/                             # Go client SDK
└── AgentDocs/                       # Development workflow
```

## Technology Stack

### Core
- **Language**: Go 1.22+
- **Database**: SQLite with WAL mode
  - Driver: `modernc.org/sqlite` (pure Go)
  - Migrations: Goose (`github.com/pressly/goose/v3`)
  - Code Generation: SQLc (`github.com/sqlc-dev/sqlc`)

### Web & API
- **HTTP Framework**: Gin (`github.com/gin-gonic/gin`)
- **REST API**: Cluster-scoped JSON API
- **Auth**: HMAC-SHA256 token hashing

### CLI & UI
- **CLI Framework**: Cobra (`github.com/spf13/cobra`)
- **TUI**: Bubble Tea (`github.com/charmbracelet/bubbletea`)

### Logging
- **Logging**: Zap (`go.uber.org/zap`)
- **Metrics**: Prometheus (optional)

## Key Components

### 1. Control Plane Server (`nebulagc-server`)
Central REST API and super-admin CLI for:
- Multi-tenant cluster management
- Node enrollment and authentication
- Config bundle distribution with versioning
- PKI storage and certificate generation
- High availability (master/replica mode)
- Lighthouse process management

### 2. Client Daemon (`nebulagc`)
Node-side daemon that:
- Manages multiple Nebula instances
- Polls for config updates (5-second intervals)
- Downloads and unpacks config bundles
- Supervises Nebula processes
- Handles high availability failover

### 3. Go SDK (`sdk/`)
Client library providing:
- Header-based authentication
- HA support (multiple control plane URLs)
- Automatic master discovery
- Typed request/response handling

## Database Schema (Planned)

- **tenants**: Organization records
- **clusters**: Nebula cluster configurations with PKI
- **cluster_state**: Per-instance lighthouse version tracking
- **replicas**: Control plane instance registry
- **nodes**: Nebula node records with auth tokens
- **config_bundles**: Versioned tar.gz archives

## Key Architectural Decisions

### High Availability
- Master/Replica Architecture (one writer, multiple readers)
- N-Way Lighthouse Redundancy
- External SQLite replication (Litestream, LiteFS, custom)
- Automatic failover via SDK master discovery

### Security Model
- Multi-layer authentication (node tokens, cluster tokens)
- HMAC-SHA256 hashing with server-side secret
- Cluster-scoped REST API (no cross-tenant operations)
- Super-admin operations via CLI/Unix socket only
- Rate limiting per-IP and per-node

### Version Management
- Incremental versioning per cluster
- Lighthouse watcher with 5-second polling
- Automatic process restart on config changes
- HTTP caching with 304 Not Modified responses

## Development Workflow

### Prerequisites
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
```

### Build Process
```bash
# Generate SQLc code
cd server && sqlc generate

# Run migrations
goose -dir server/migrations sqlite3 ./nebula.db up

# Build binaries
go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
go build -o bin/nebulagc ./cmd/nebulagc
```

### Environment Variables
- `NEBULAGC_HMAC_SECRET`: Token hashing key (required, 32+ bytes)
- `NEBULAGC_LOG_LEVEL`: debug/info/warn/error
- `NEBULAGC_LOG_FORMAT`: json/console
- `NEBULAGC_ADMIN_SOCKET_PATH`: Unix socket path
- `NEBULAGC_INSTANCE_ID`: Control plane instance UUID
- `NEBULAGC_CONFIG_CHECK_INTERVAL`: Lighthouse polling interval (default: 5s)

## Implementation Phases

### Phase 1: HA Control Plane Core
- Master/replica runtime
- Cluster/node CRUD operations
- Token hashing and validation
- Bundle upload/download with versioning
- Lighthouse watcher
- Database migrations and models
- Basic CLI with Bubble Tea output

### Phase 2: Node-Facing SDK and Daemon
- Complete Go SDK with HA support
- Multi-cluster daemon with polling
- Bundle unpacking and Nebula restart
- Route management
- MTU updates
- Token rotation flows

### Phase 3: Ops Hardening and Tooling
- Rate limiting
- Structured logging with Zap
- Authentication failure telemetry
- Deployment documentation
- Development tooling (Makefile, linting, tests)
- Cleanup utilities

## Key Documentation Files

### [README.md](README.md)
Project overview, features, and quick start guide

### [AgentDocs/ToDo/nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md)
Comprehensive technical specification (1,386 lines) containing:
- Complete REST API specification (30+ endpoints)
- Database schema definitions
- Authentication flows
- High availability architecture
- Token security requirements
- Client SDK specification
- Daemon design
- Development workflow

## Working with This Project

### Current State
The project is in the **planning/specification phase**. There is no implementation code yet, only comprehensive documentation defining the system architecture and requirements.

### Starting Development
1. Review [nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md) for detailed specifications
2. Set up development environment with Go 1.22+ and required tools
3. Begin with Phase 1 implementation (HA Control Plane Core)
4. Follow the database schema and API specifications exactly
5. Use AgentDocs folders to track implementation progress

### Code Standards
- Follow Go best practices and idioms
- Use structured logging (Zap)
- Generate database code with SQLc
- Write tests for all business logic
- Document security-critical code thoroughly

## API Overview

The REST API will be cluster-scoped with 30+ endpoints organized into these categories:

- **Node Management**: Create, list, delete, rotate tokens
- **Config Distribution**: Version checks, bundle upload/download
- **Topology Management**: Lighthouse/relay assignments
- **Route Management**: Internal network advertisement
- **Token Rotation**: Node and cluster token security
- **Health/Status**: Master checks, readiness, metrics

All API requests use header-based authentication:
- `X-NebulaGC-Cluster-Token`: Cluster-scoped operations
- `X-NebulaGC-Node-Token`: Node-scoped operations

## Security Considerations

- All tokens are 41+ characters
- HMAC-SHA256 hashing with constant-time comparison
- Generic error messages to prevent information disclosure
- Rate limiting on authentication endpoints
- Cluster-scoped isolation (no cross-tenant access)
- Super-admin operations isolated to CLI/Unix socket

## Testing Strategy

### Unit Tests
- Database operations (SQLc generated code)
- Authentication/authorization logic
- Token hashing and validation
- Business logic in service layer

### Integration Tests
- Full API endpoint testing
- HA failover scenarios
- Multi-cluster daemon behavior
- Config versioning and distribution

### End-to-End Tests
- Full deployment scenarios
- Node enrollment workflows
- Certificate generation and distribution
- Process supervision and restart

## Deployment

### Systemd Service
Run as system daemon with automatic restart

### Docker Container
Alpine-based minimal image with volume mounts

### Configuration
- Server: CLI flags and environment variables
- Daemon: `/etc/nebulagc/config.json` (multi-cluster)
- Development: `dev_config.json` (git-ignored)

## Contributing

### Task Management Workflow

**CRITICAL**: All tasks must follow the numbered task system for tracking and rollback capability.

#### Starting New Work
1. Identify the task in `AgentDocs/ToDo/` (unnumbered specification file)
2. Check `AgentDocs/Done/` to find the highest task number (e.g., `00003_...`)
3. Move task to `InProgress/` with next sequential number:
   ```
   AgentDocs/ToDo/implement_cluster_service.md
   → AgentDocs/InProgress/00004_implement_cluster_service.md
   ```
4. Update task file with start date and planned changes

#### Completing Work
1. Ensure all requirements in [constitution.md](AgentDocs/constitution.md) are met:
   - All functions documented
   - All structs documented
   - Tests written and passing
   - Models in separate package
2. Move task to `Done/` keeping the same number:
   ```
   AgentDocs/InProgress/00004_implement_cluster_service.md
   → AgentDocs/Done/00004_implement_cluster_service.md
   ```
3. Update task file with completion date
4. Commit with message referencing task number

#### Rolling Back Changes
The numbered system enables easy rollback:
1. Identify task to undo (e.g., `00004_implement_cluster_service.md`)
2. Revert code changes made in that task
3. Reverse database migrations if any
4. Move task file back to `InProgress/` or `ToDo/`
5. Continue with previous task number sequence

### Implementation Guidelines
1. Follow the specification in [nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md)
2. Adhere to all standards in [constitution.md](AgentDocs/constitution.md)
3. Write tests alongside implementation
4. Document all code (functions, structs, fields)
5. Use shared `models/` package for all data structures
6. Update task files with progress

## Resources

- **Nebula Project**: https://github.com/slackhq/nebula
- **SQLc Documentation**: https://docs.sqlc.dev/
- **Goose Migrations**: https://github.com/pressly/goose
- **Gin Framework**: https://gin-gonic.com/
- **Bubble Tea TUI**: https://github.com/charmbracelet/bubbletea

## Questions or Issues?

Refer to the comprehensive specification in [nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md) for detailed answers about:
- API endpoint specifications
- Database schema details
- Authentication flows
- High availability behavior
- Token security requirements
- Deployment configurations
