# Task 00005: REST API Foundation

**Status**: Completed
**Started**: 2025-01-21
**Completed**: 2025-01-21
**Phase**: 1 (HA Control Plane Core)
**Dependencies**: Task 00004 (Authentication and Token Management)

## Objective

Implement the REST API foundation using Gin framework with:
- HTTP router with all route definitions
- Authentication middleware (cluster token and node token)
- Rate limiting middleware
- Replica write guard middleware (HA support)
- Health check endpoints
- Error response formatting
- Request logging with Zap
- CORS middleware

## Scope

### Files to Create
- `server/internal/api/router.go` - Main router setup and route registration
- `server/internal/api/middleware/auth.go` - Authentication middleware
- `server/internal/api/middleware/ratelimit.go` - Rate limiting middleware
- `server/internal/api/middleware/replica.go` - Write guard for replicas
- `server/internal/api/middleware/logging.go` - Request logging with Zap
- `server/internal/api/middleware/cors.go` - CORS configuration
- `server/internal/api/handlers/health.go` - Health check handlers
- `server/internal/api/handlers/common.go` - Common response helpers
- `server/internal/api/context.go` - Request context helpers
- `server/cmd/nebulagc-server/main.go` - Server entrypoint

### Route Groups Defined
- `/health` - Health checks (no auth)
- `/api/v1/nodes` - Node management (node token auth)
- `/api/v1/config` - Config distribution (node token auth)
- `/api/v1/topology` - Topology management (cluster token auth)
- `/api/v1/routes` - Route management (node token auth)
- `/api/v1/tokens` - Token rotation (varies)

## Implementation Details

### Authentication Middleware
Uses the token package from Task 00004:
- Extracts tokens from headers (`X-NebulaGC-Cluster-Token`, `X-NebulaGC-Node-Token`)
- Validates length using `token.ValidateLength()`
- Queries database for stored hash
- Validates using `token.Validate()` with constant-time comparison
- Sets authenticated context (tenant, cluster, node)

### Rate Limiting
- Per-IP rate limiting using token bucket algorithm
- Per-node rate limiting (after authentication)
- Different limits for auth vs non-auth endpoints
- 429 Too Many Requests on limit exceeded

### Replica Write Guard
- Checks if instance is master (queries `replicas` table)
- Allows all GET/HEAD requests
- Blocks POST/PUT/DELETE on replicas
- Returns 503 Service Unavailable with master URL

### Error Response Format
```json
{
  "error": "error_code",
  "message": "Human-readable message",
  "request_id": "uuid"
}
```

### Health Check Endpoints
- `GET /health/live` - Liveness probe (always 200)
- `GET /health/ready` - Readiness probe (checks DB connection)
- `GET /health/master` - Master status (returns master URL)

## Standards Compliance

- ✅ All functions have documentation comments
- ✅ All structs and fields documented
- ✅ Errors wrapped with context
- ✅ Uses shared models package
- ✅ Zap structured logging
- ✅ No code duplication
- ✅ Generic error messages (no information disclosure)

## Testing Strategy

- Unit tests for each middleware
- Mock HTTP requests with httptest
- Verify authentication flows
- Test rate limiting behavior
- Test replica write blocking
- Integration tests in later task

## Progress

- [x] Create API package structure
- [x] Implement authentication middleware
- [x] Implement rate limiting middleware
- [x] Implement replica write guard
- [x] Implement logging middleware
- [x] Implement CORS middleware
- [x] Implement health check handlers
- [x] Implement common response helpers
- [x] Create router with all route groups
- [x] Create server entrypoint
- [x] Build and verify server binary
- [x] Update task documentation

## Implementation Summary

### Files Created (10 files, ~1,200 lines)

1. **server/internal/api/context.go** (102 lines)
   - Context key constants for authenticated request data
   - Helper functions for getting/setting tenant, cluster, node IDs
   - Request ID management

2. **server/internal/api/handlers/common.go** (144 lines)
   - Standardized error/success response structures
   - Error response helpers with request ID tracking
   - Error mapping from models package to HTTP responses
   - Generic error messages for security

3. **server/internal/api/handlers/health.go** (108 lines)
   - HealthHandler with liveness, readiness, master checks
   - Database connectivity verification
   - Master status determination for HA

4. **server/internal/api/middleware/logging.go** (72 lines)
   - Request logging with Zap structured logging
   - Automatic request ID generation (UUID)
   - Logs method, path, status, duration, client IP
   - Includes authenticated context (tenant/cluster/node IDs)

5. **server/internal/api/middleware/cors.go** (47 lines)
   - CORS header management
   - Origin whitelisting
   - Preflight request handling

6. **server/internal/api/middleware/auth.go** (196 lines)
   - RequireClusterToken middleware
   - RequireNodeToken middleware
   - RequireAdminNode middleware
   - Uses token package for validation with constant-time comparison
   - Sets authenticated context on success
   - Generic error messages to prevent enumeration

7. **server/internal/api/middleware/ratelimit.go** (161 lines)
   - Token bucket rate limiter implementation
   - RateLimitByIP (100 req/s, burst 200)
   - RateLimitByNode (50 req/s, burst 100)
   - RateLimitByCluster (100 req/s, burst 200)
   - Automatic cleanup of expired limiters

8. **server/internal/api/middleware/replica.go** (129 lines)
   - WriteGuard middleware blocks writes on replicas
   - Master determination by oldest healthy replica
   - Returns master URL for client failover
   - Allows all GET/HEAD requests on replicas

9. **server/internal/api/router.go** (202 lines)
   - SetupRouter with all middleware chain
   - Health check routes (no auth)
   - Node management routes (node token + rate limit)
   - Config distribution routes (node token + lower rate limit)
   - Topology management routes (cluster token + rate limit)
   - Route management routes (node token + rate limit)
   - Placeholder for token rotation routes

10. **server/cmd/nebulagc-server/main.go** (245 lines)
    - Command-line flag parsing
    - Environment variable support
    - Logger setup (json/console format, configurable level)
    - Database connection with WAL mode
    - Router initialization
    - HTTP server startup
    - Configuration validation (HMAC secret, instance ID)

### Dependencies Added
- `github.com/gin-gonic/gin` - HTTP framework
- `github.com/google/uuid` - UUID generation for request IDs
- `go.uber.org/zap` - Structured logging
- `golang.org/x/time/rate` - Rate limiting
- `modernc.org/sqlite` - Pure Go SQLite driver

### Security Features Implemented
- ✅ Constant-time token validation (prevents timing attacks)
- ✅ Generic error messages (prevents information disclosure)
- ✅ Rate limiting by IP, node, and cluster
- ✅ HMAC-SHA256 token hashing
- ✅ Request ID tracking for audit trails
- ✅ Replica write guard for HA consistency

### Build Verification
```
$ go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
$ ls -lh bin/nebulagc-server
-rwxr-xr-x@ 1 yaroslav  staff   16M Nov 21 15:30 bin/nebulagc-server

$ ./bin/nebulagc-server -help
Usage of ./bin/nebulagc-server:
  -cors-origins string
        Comma-separated list of allowed CORS origins (* for all)
  -db string
        Path to SQLite database file (default "./nebula.db")
  -disable-write-guard
        Disable replica write guard (single-instance mode)
  -instance-id string
        Control plane instance UUID (auto-generated if not provided)
  -listen string
        Address to listen on (default ":8080")
  -log-format string
        Log format (json, console) (default "console")
  -log-level string
        Log level (debug, info, warn, error) (default "info")
  -secret string
        HMAC secret for token validation (required, min 32 bytes)
```

## Notes

This task establishes the complete HTTP layer foundation for the control plane. Key accomplishments:

1. **Authentication Integration**: Successfully integrated the token package from Task 00004 into authentication middleware
2. **HA Architecture**: Implemented replica write guard that enforces master-only writes
3. **Rate Limiting**: Three-tier rate limiting (IP, node, cluster) prevents abuse
4. **Structured Logging**: Zap integration with request IDs for tracing
5. **Security-First Design**: Generic errors, constant-time comparison, no information disclosure

The router defines all route groups with placeholders for handlers that will be implemented in:
- Task 00007: Node management handlers
- Task 00008: Config bundle handlers
- Task 00009: Topology and route handlers

The server is fully buildable and can start (though actual handler logic will return 404 until subsequent tasks are completed).
