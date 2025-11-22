# Changelog

All notable changes to NebulaGC will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Client daemon (`nebulagc`) for node-side management
- Multi-cluster support per daemon instance
- Automatic Nebula process supervision
- Config polling and auto-update mechanism

## [0.9.0] - 2025-01-22 (v1.0.0 Release Candidate)

### Added - Phase 3: Production Hardening

**Security & Observability:**
- Advanced rate limiting (per-IP, per-node, per-cluster strategies)
- Structured logging with Zap (JSON and console formats)
- Prometheus metrics endpoint (`/metrics`)
- Security audit and hardening (input validation, HMAC secrets, error sanitization)

**Testing & Quality:**
- End-to-end testing infrastructure (48 tests in 260ms)
- Performance benchmarking framework
- Database-level integration tests
- API-level integration tests
- Test fixtures and helpers
- Makefile targets for test execution (`make test-e2e`, `make test-all`)

**Operations:**
- Deployment documentation (systemd, Docker, Kubernetes)
- High availability setup guides
- Litestream and LiteFS replication guides
- Operational utilities (replica pruning, bundle verification, DB compaction)
- Development tooling (Makefile, linting, CI integration)

**Documentation:**
- Comprehensive README with current status
- Performance benchmarking guide
- Optimization recommendations
- API examples and usage patterns

### Changed
- PROGRESS.md updated to 90% complete (9/10 Phase 3 tasks)
- Enhanced error handling across all layers
- Improved logging with request tracking
- Optimized database queries for performance

### Security
- HMAC-SHA256 token hashing with constant-time comparison
- Minimum 41-character token length enforcement
- Rate limiting prevents brute force attacks
- Generic error messages prevent information disclosure
- Input validation on all API endpoints

## [0.8.0] - 2025-01-15

### Added - Phase 1: HA Control Plane Core

**Core Infrastructure:**
- Multi-tenant architecture (tenants, clusters, nodes)
- SQLite database with WAL mode
- Database migrations system (7 migrations)
- SQLc for type-safe query generation
- Shared models package

**Authentication & Authorization:**
- Cluster-scoped authentication
- Node-level authentication
- Admin node privileges
- Token generation with cryptographic randomness
- HMAC-SHA256 token hashing
- Token rotation support

**REST API:**
- Health endpoints (liveness, readiness, master status)
- Node management (CRUD, MTU updates, token rotation)
- Config bundle distribution (upload, download, versioning)
- Topology management (routes, lighthouses, relays)
- Cluster token rotation
- Request logging middleware
- CORS middleware
- Authentication middleware
- Rate limiting middleware (basic)

**High Availability:**
- Master/replica mode support
- Replica registry with heartbeat mechanism
- Automatic master election (oldest-first algorithm)
- Replica pruning (stale detection)
- Health check propagation
- Read replication support

**Server CLI:**
- Master/replica mode selection
- Configuration via flags and environment variables
- Instance ID generation
- HMAC secret management
- Graceful shutdown handling

**Lighthouse Management:**
- Background config version watcher
- Nebula process spawning and supervision
- Config generation from database
- Process restart on config changes
- State tracking in cluster_state table

### Technical Details
- **API Endpoints**: 20+ functional endpoints
- **Database Tables**: 8 tables with comprehensive indexing
- **Code Coverage**: Comprehensive unit and integration tests
- **Lines of Code**: ~5,000+ lines
- **Test Coverage**: 58+ tests passing

## [0.5.0] - 2025-01-10

### Added - Project Foundation

**Planning & Design:**
- Complete technical specification (1,386 lines)
- 32-task implementation roadmap
- Three-phase development plan
- Coding standards and constitution
- Task breakdown for all phases

**Project Structure:**
- Go workspace with modules
- Server module structure
- SDK module structure
- Client daemon module structure
- Models package
- Shared utilities package

**Development Infrastructure:**
- Makefile for build automation
- golangci-lint configuration
- Git workflow documentation
- AI agent guide (claude.md)
- Task management system (AgentDocs/)

## Version History Summary

- **v0.9.0** (2025-01-22): Production hardening complete (90% Phase 3)
- **v0.8.0** (2025-01-15): HA control plane core complete (Phase 1)
- **v0.5.0** (2025-01-10): Planning and project structure

## Upgrade Notes

### From v0.8.0 to v0.9.0
- No breaking changes
- New Prometheus metrics endpoint available at `/metrics`
- Environment variables added for log format and level
- E2E tests can be run with `make test-e2e`
- Benchmarking infrastructure available in `tests/bench/`

### From v0.5.0 to v0.8.0
- Database migrations applied automatically on startup
- HMAC secret now required (set `NEBULAGC_HMAC_SECRET`)
- Public URL configuration required for replicas
- See deployment documentation for detailed setup

## Future Releases

### v1.0.0 (Planned: 2025-01-25)
- **Final documentation completion** (Task 00032)
- Getting started guide
- Complete API reference
- Operations manual
- Architecture documentation
- v1.0.0 stable release

### v1.1.0 (Planned: TBD)
- **Client daemon implementation** (Task 00012 and beyond)
- Multi-cluster daemon support
- Automatic config polling
- Nebula process supervision
- Complete Phase 2 functionality

## Links

- [GitHub Repository](https://github.com/yaroslav-gwit/nebulagc)
- [Technical Specification](AgentDocs/Planning/nebula_control_plane_spec.md)
- [Implementation Roadmap](AgentDocs/Planning/implementation_roadmap.md)
- [Progress Tracking](PROGRESS.md)

[Unreleased]: https://github.com/yaroslav-gwit/nebulagc/compare/v0.9.0...HEAD
[0.9.0]: https://github.com/yaroslav-gwit/nebulagc/releases/tag/v0.9.0
[0.8.0]: https://github.com/yaroslav-gwit/nebulagc/releases/tag/v0.8.0
[0.5.0]: https://github.com/yaroslav-gwit/nebulagc/releases/tag/v0.5.0
