# Phase 3: Ops Hardening and Tooling - Task Breakdown

## Overview
Phase 3 focuses on production readiness: rate limiting, structured logging, metrics, deployment documentation, development tooling, and operational utilities.

---

## Task 00023: Advanced Rate Limiting

**Objective**: Implement production-grade rate limiting with per-IP and per-node strategies.

**Deliverables**:
- `server/internal/ratelimit/limiter.go` - Rate limiting implementation
- `server/internal/ratelimit/storage.go` - In-memory storage with TTL
- `server/internal/api/middleware/ratelimit.go` - Middleware integration
- Token bucket algorithm
- Separate limits for auth failures and authenticated requests
- Configurable via environment variables

**Rate Limit Rules**:
- **Authentication failures** (per IP):
  - 10 failures per minute → 429 with `Retry-After: 60`
  - 50 failures in 10 minutes → 1 hour block
- **Authenticated requests** (per node):
  - 100 requests per minute for version checks
  - 10 bundle uploads per cluster per minute
- **Unauthenticated endpoints** (per IP):
  - 30 requests per minute for health checks

**Configuration**:
- `NEBULAGC_RATELIMIT_AUTH_FAILURES_PER_MIN` (default: 10)
- `NEBULAGC_RATELIMIT_AUTH_FAILURES_BLOCK_MIN` (default: 60)
- `NEBULAGC_RATELIMIT_REQUESTS_PER_MIN` (default: 100)
- `NEBULAGC_RATELIMIT_BUNDLE_UPLOADS_PER_MIN` (default: 10)

**Dependencies**: Phase 1 complete (API middleware)

**Testing**:
- Rate limits enforced correctly
- 429 responses include Retry-After header
- Limits reset after time window
- Different limits for different endpoints
- Per-node vs per-IP tracking works

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00024: Structured Logging Enhancements

**Objective**: Improve logging with consistent fields, log levels, and operational visibility.

**Deliverables**:
- `server/internal/logging/logger.go` - Zap logger configuration
- `server/internal/logging/middleware.go` - Request logging middleware
- `server/internal/logging/fields.go` - Standard field helpers
- Production (JSON) and development (console) modes
- Configurable log levels
- Sampling for high-volume logs

**Standard Fields**:
- `tenant_id`, `cluster_id`, `node_id` - Resource identifiers
- `request_id` - UUID per request for tracing
- `source_ip` - Client IP address
- `duration_ms` - Request duration
- `status_code` - HTTP response status
- `method`, `path` - HTTP method and path

**Log Levels**:
- **DEBUG**: Development debugging (verbose)
- **INFO**: Normal operations (node created, bundle uploaded, etc.)
- **WARN**: Authentication failures, rate limiting, degraded mode
- **ERROR**: Server errors, database failures, process crashes
- **FATAL**: Startup failures, configuration errors

**Sampling**:
- High-frequency logs (version checks) sampled at 1:100 at INFO
- All errors logged without sampling

**Configuration**:
- `NEBULAGC_LOG_LEVEL` (debug, info, warn, error)
- `NEBULAGC_LOG_FORMAT` (json, console)
- `NEBULAGC_LOG_SAMPLING` (true, false)

**Dependencies**: Phase 1 complete (logging foundation)

**Testing**:
- Logs output in correct format (JSON vs console)
- Log levels filter correctly
- Standard fields present in all logs
- Sampling reduces volume for high-frequency logs
- Sensitive data never logged (tokens, keys)

**Estimated Complexity**: Low
**Priority**: Medium

---

## Task 00025: Prometheus Metrics

**Objective**: Expose Prometheus metrics for monitoring and alerting.

**Deliverables**:
- `server/internal/metrics/metrics.go` - Metrics definitions
- `server/internal/api/handlers/metrics.go` - Metrics endpoint
- Request latency histograms
- Request count by endpoint and status
- Authentication failure counters
- Bundle upload/download counters
- Lighthouse process health gauges

**Metrics**:
- `nebulagc_http_requests_total{method, path, status}` - Request counter
- `nebulagc_http_request_duration_seconds{method, path}` - Latency histogram
- `nebulagc_auth_failures_total{reason}` - Auth failure counter
- `nebulagc_bundle_uploads_total{cluster_id}` - Bundle upload counter
- `nebulagc_bundle_downloads_total{cluster_id}` - Bundle download counter
- `nebulagc_lighthouse_processes{cluster_id, status}` - Lighthouse health
- `nebulagc_config_version{cluster_id}` - Current config version
- `nebulagc_replica_count` - Number of registered replicas

**Endpoint**:
- `GET /metrics` - Prometheus scrape endpoint
- Optional authentication via `--metrics-auth` flag
- Optional separate port via `--metrics-port` flag

**Dependencies**: Phase 1 complete (API handlers)

**Testing**:
- Metrics endpoint returns valid Prometheus format
- Metrics update on requests
- Authentication failures increment counter
- Lighthouse metrics reflect process state

**Estimated Complexity**: Low-Medium
**Priority**: Medium

---

## Task 00026: Deployment Documentation

**Objective**: Create comprehensive deployment guides for systemd, Docker, and Kubernetes.

**Deliverables**:
- `docs/deployment/systemd.md` - Systemd deployment guide
- `docs/deployment/docker.md` - Docker deployment guide
- `docs/deployment/kubernetes.md` - Kubernetes deployment guide
- `docs/deployment/litestream.md` - SQLite replication with Litestream
- `docs/deployment/litefs.md` - SQLite replication with LiteFS
- `docs/deployment/ha-setup.md` - High availability setup guide
- Example configuration files for all platforms

**Systemd Guide**:
- Service unit files for server and daemon
- Environment variable configuration
- Log management with journald
- Auto-restart configuration
- Security hardening (User, Group, NoNewPrivileges, etc.)

**Docker Guide**:
- Dockerfile for server
- Dockerfile for daemon
- Multi-stage builds for minimal images
- Volume mounts for database and config
- docker-compose.yml for local testing
- Production considerations (secrets, networking)

**Kubernetes Guide**:
- Deployment manifests (server, daemon)
- Service definitions
- ConfigMap and Secret usage
- StatefulSet for server (database persistence)
- DaemonSet for daemon (one per node)
- Liveness and readiness probes
- HA considerations (multiple master replicas)

**SQLite Replication Guides**:
- Litestream setup for S3/GCS/Azure backup
- LiteFS setup for distributed SQLite
- Replication lag considerations
- Backup and restore procedures

**Dependencies**: None (documentation)

**Testing**:
- Follow each guide manually
- Verify deployments work
- Test HA failover scenarios

**Estimated Complexity**: Medium
**Priority**: High

---

## Task 00027: Development Tooling

**Objective**: Create Makefile, linting, testing, and CI configuration.

**Deliverables**:
- `Makefile` - Common development tasks
- `.golangci.yml` - golangci-lint configuration
- `.github/workflows/ci.yml` - GitHub Actions CI
- `scripts/dev-setup.sh` - Development environment setup
- `scripts/generate-dev-certs.sh` - Generate test certificates
- `scripts/seed-dev-data.sh` - Seed database with test data

**Makefile Targets**:
- `make build` - Build all binaries
- `make test` - Run all tests
- `make test-coverage` - Generate coverage report
- `make lint` - Run linters
- `make format` - Format code
- `make generate` - Run sqlc and other code generators
- `make migrate` - Apply database migrations
- `make dev-server` - Start development server
- `make dev-daemon` - Start development daemon
- `make clean` - Clean build artifacts

**Linting**:
- golangci-lint with recommended linters
- Custom rules for error handling, logging, security
- Pre-commit hook integration

**CI Pipeline**:
- Run on pull requests and main branch
- Go version matrix (1.22, 1.23)
- Run tests with race detector
- Run linters
- Generate code coverage report
- Upload coverage to Codecov

**Dependencies**: All phases complete

**Testing**:
- All Make targets work
- Linting catches common issues
- CI pipeline runs successfully

**Estimated Complexity**: Low-Medium
**Priority**: Medium

---

## Task 00028: Operational Utilities

**Objective**: Create utilities for database maintenance, replica health checks, and troubleshooting.

**Deliverables**:
- `server/cmd/nebulagc-server/cmd/util.go` - Utility subcommands
- Replica pruning utility
- Bundle verification utility
- Database compaction utility
- Lighthouse health check utility
- Token hash verification utility

**Utilities**:

1. **Replica Pruning**:
   - `nebulagc-server util prune-replicas [--dry-run]`
   - Remove stale replicas (no heartbeat for >5 minutes)
   - Show what would be deleted (dry-run mode)

2. **Bundle Verification**:
   - `nebulagc-server util verify-bundles --cluster-id <id>`
   - Verify all bundles in database are valid tar.gz
   - Check required files present
   - Report corruption

3. **Database Compaction**:
   - `nebulagc-server util compact-db`
   - Run SQLite VACUUM
   - Rebuild indexes
   - Report space savings

4. **Lighthouse Health Check**:
   - `nebulagc-server util check-lighthouses [--cluster-id <id>]`
   - Verify all instances running lighthouses
   - Check version consistency
   - Report any stopped processes

5. **Token Hash Verification**:
   - `nebulagc-server util verify-token --node-id <id> --token <token>`
   - Test if a token matches stored hash
   - Useful for troubleshooting auth issues

**Dependencies**: Phase 1 and Phase 2 complete

**Testing**:
- Each utility performs correct operation
- Dry-run modes don't modify data
- Error handling for edge cases

**Estimated Complexity**: Low-Medium
**Priority**: Low

---

## Task 00029: Security Audit and Hardening

**Objective**: Conduct security review and implement additional hardening measures.

**Deliverables**:
- `docs/security/threat-model.md` - Threat model documentation
- `docs/security/security-checklist.md` - Deployment security checklist
- Security audit report
- Implement security recommendations
- Update code to address findings

**Security Review Areas**:
- Authentication and authorization
- Token generation and storage
- Input validation and sanitization
- SQL injection prevention (SQLc parameterization)
- SSRF prevention (bundle downloads)
- Path traversal prevention (config directories)
- Information disclosure (error messages)
- Rate limiting effectiveness
- Logging of sensitive data

**Hardening Measures**:
- HTTPS enforcement (optional TLS for server)
- TLS certificate validation for control plane URLs
- Filesystem permissions for config directories
- Process isolation (user/group separation)
- Resource limits (memory, CPU, file descriptors)
- Secure defaults (HMAC secret generation)

**Threat Model**:
- Document attack vectors
- Define security boundaries
- Identify high-risk areas
- Recommend mitigations

**Dependencies**: All implementation complete

**Testing**:
- Penetration testing (automated and manual)
- Fuzzing for input validation
- Security scanner runs (gosec, staticcheck)

**Estimated Complexity**: Medium-High
**Priority**: High

---

## Task 00030: End-to-End Testing and Validation

**Objective**: Create comprehensive end-to-end tests covering all major workflows.

**Deliverables**:
- `tests/e2e/` - End-to-end test suite
- `tests/e2e/fixtures/` - Test data and configurations
- `tests/e2e/scenarios/` - Test scenarios
- Docker Compose setup for E2E tests
- CI integration for E2E tests

**Test Scenarios**:

1. **Single Node Enrollment**:
   - Start control plane
   - Create tenant and cluster
   - Create admin node
   - Start daemon with admin credentials
   - Daemon downloads config bundle
   - Nebula process starts successfully

2. **Multi-Cluster Management**:
   - Configure daemon with 3 clusters
   - Verify all 3 Nebula processes running
   - Update one cluster config
   - Verify only that cluster restarts

3. **HA Failover**:
   - Start master and 2 replicas
   - Daemon connects to all 3
   - Kill master
   - Verify failover to replica for reads
   - Writes fail gracefully
   - Restart master
   - Verify writes resume

4. **Lighthouse Redundancy**:
   - Create cluster with `provide_lighthouse=true`
   - Start master and 2 replicas
   - Verify 3 lighthouse processes running (one per instance)
   - Update config_version
   - Verify all 3 lighthouses restart

5. **Token Rotation**:
   - Create node with token
   - Rotate node token
   - Verify old token fails auth
   - Update daemon config with new token
   - Verify daemon continues working

6. **Bundle Update Propagation**:
   - Upload new bundle
   - Verify config_version increments
   - Daemon polls and detects new version
   - Bundle downloaded and unpacked
   - Nebula process restarts with new config

**Dependencies**: All implementation and deployment docs complete

**Testing**:
- All scenarios pass
- Tests run in CI
- Flakiness < 1%
- Average run time < 5 minutes

**Estimated Complexity**: High
**Priority**: High

---

## Task 00031: Performance Testing and Optimization

**Objective**: Benchmark performance and optimize bottlenecks.

**Deliverables**:
- `tests/bench/` - Benchmark suite
- Performance testing documentation
- Optimization recommendations
- Implement high-impact optimizations

**Benchmarks**:

1. **API Throughput**:
   - Requests per second for version checks
   - Requests per second for bundle downloads
   - Concurrent connection handling

2. **Database Performance**:
   - Query latency (p50, p95, p99)
   - Write throughput (node creation, bundle upload)
   - Read throughput (node listing, topology queries)

3. **Lighthouse Management**:
   - Time to restart lighthouse after config change
   - Memory usage per lighthouse process
   - CPU usage during normal operation

4. **Daemon Polling**:
   - Overhead of 5-second polling
   - Memory usage for multi-cluster daemon
   - CPU usage during bundle updates

**Optimization Targets**:
- API latency p95 < 100ms
- Bundle download < 1s for 10MB bundle
- Lighthouse restart < 5s after config change
- Daemon memory < 50MB per cluster
- Support 1000+ nodes per cluster
- Support 100+ clusters per tenant

**Dependencies**: All implementation complete

**Testing**:
- Benchmarks run consistently
- Performance regressions detected in CI
- Optimization improves metrics

**Estimated Complexity**: Medium-High
**Priority**: Medium

---

## Task 00032: Documentation Finalization

**Objective**: Complete all user-facing and developer documentation.

**Deliverables**:
- `docs/README.md` - Documentation index
- `docs/getting-started.md` - Quick start guide
- `docs/architecture.md` - Architecture overview
- `docs/api-reference.md` - Complete API reference
- `docs/sdk-guide.md` - SDK usage guide
- `docs/daemon-guide.md` - Daemon setup and usage
- `docs/operations.md` - Operational runbook
- `docs/troubleshooting.md` - Troubleshooting guide
- `docs/faq.md` - Frequently asked questions
- `CHANGELOG.md` - Version history
- `CONTRIBUTING.md` - Contribution guidelines

**Documentation Quality**:
- Clear, concise writing
- Code examples for all common tasks
- Screenshots where helpful
- Troubleshooting for common issues
- Links to related sections
- Up-to-date with implementation

**Dependencies**: All implementation complete

**Testing**:
- Documentation reviewed by external user
- All examples tested and working
- Links valid (no 404s)

**Estimated Complexity**: Medium
**Priority**: Medium

---

## Phase 3 Completion Criteria

All tasks completed when:
- ✅ Rate limiting enforced in production
- ✅ Structured logging provides operational visibility
- ✅ Prometheus metrics available for monitoring
- ✅ Deployment guides complete and tested
- ✅ Development tooling fully functional
- ✅ Operational utilities available
- ✅ Security audit complete and findings addressed
- ✅ End-to-end tests passing
- ✅ Performance benchmarks meet targets
- ✅ Documentation complete and reviewed
- ✅ Ready for v1.0.0 release

---

## Task Dependencies Diagram

```
Phase 2 Complete (00022)
  ├─→ 00023 (Rate Limiting)
  ├─→ 00024 (Logging Enhancements)
  ├─→ 00025 (Prometheus Metrics)
  ├─→ 00026 (Deployment Docs)
  ├─→ 00027 (Dev Tooling)
  ├─→ 00028 (Operational Utilities)
  ├─→ 00029 (Security Audit)
  ├─→ 00030 (E2E Testing)
  ├─→ 00031 (Performance Testing)
  └─→ 00032 (Documentation Finalization)
```

---

## Validation Process

For each task:
1. Move task file from `ToDo/` to `InProgress/` with next sequential number
2. Implement according to constitution standards
3. Document all functions, structs, and fields
4. Write tests where applicable
5. Ensure all tests pass
6. Create git commit referencing task number
7. Move task file to `Done/` keeping same number
8. Update task file with completion date

---

## Release Readiness

After Phase 3 completion:
- Tag version v1.0.0
- Publish binaries (GitHub Releases)
- Publish Docker images
- Announce release
- Monitor production deployments
- Gather feedback for v1.1.0
