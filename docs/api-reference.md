# NebulaGC API Reference

Complete reference for the NebulaGC REST API v1.

## Table of Contents

- [Authentication](#authentication)
- [Common Responses](#common-responses)
- [Health and Version Endpoints](#health-and-version-endpoints)
- [Node Management](#node-management)
- [Config Bundle Management](#config-bundle-management)
- [Topology Management](#topology-management)
- [Rate Limiting](#rate-limiting)
- [Error Codes](#error-codes)

## Authentication

All endpoints (except `/health` and `/version`) require authentication via Bearer token.

### Request Headers

```http
Authorization: Bearer <node_auth_token>
Content-Type: application/json
```

### Token Lifecycle

1. **Generation**: Admin creates node via API, receives plaintext token (only time visible)
2. **Storage**: Server stores HMAC-SHA256 hash of token (never plaintext)
3. **Usage**: Node includes token in Authorization header for all requests
4. **Validation**: Server hashes provided token and compares with stored hash
5. **Rotation**: Admin can regenerate token via `/nodes/:id/token` endpoint

### Example

```bash
# Create node and receive token
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin-token" \
  -d '{
    "cluster_id": "cluster-uuid",
    "name": "node-001",
    "is_lighthouse": false,
    "overlay_ip": "10.42.0.5"
  }'

# Response includes token (save it!)
{
  "id": "node-uuid",
  "auth_token": "abc123def456...",
  ...
}

# Use token for subsequent requests
curl -X GET http://localhost:8080/api/v1/nodes/node-uuid \
  -H "Authorization: Bearer abc123def456..."
```

## Common Responses

### Success Response

```json
{
  "id": "uuid",
  "field1": "value1",
  "field2": "value2",
  "created_at": "2025-11-22T10:30:45Z",
  "updated_at": "2025-11-22T10:30:45Z"
}
```

### Error Response

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

### HTTP Status Codes

- `200 OK` - Successful GET, PUT, PATCH
- `201 Created` - Successful POST (resource created)
- `204 No Content` - Successful DELETE
- `400 Bad Request` - Invalid input
- `401 Unauthorized` - Missing or invalid authentication
- `403 Forbidden` - Insufficient permissions
- `404 Not Found` - Resource not found
- `409 Conflict` - Resource conflict (e.g., duplicate name)
- `429 Too Many Requests` - Rate limit exceeded
- `500 Internal Server Error` - Server error

## Health and Version Endpoints

### GET /health

Health check endpoint for load balancers and monitoring.

**Authentication**: None required

**Response**: 200 OK

```json
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:30:45Z",
  "version": "0.9.0",
  "ha_role": "master"
}
```

**Usage**:

```bash
curl http://localhost:8080/health
```

### GET /version

Get server version information.

**Authentication**: None required

**Response**: 200 OK

```json
{
  "version": "0.9.0",
  "commit": "abc123def",
  "build_date": "2025-11-22T10:00:00Z",
  "go_version": "go1.22.0"
}
```

**Usage**:

```bash
curl http://localhost:8080/version
```

## Node Management

### POST /api/v1/nodes

Create a new Nebula node.

**Authentication**: Required

**Request Body**:

```json
{
  "cluster_id": "cluster-uuid",
  "name": "node-001",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.5",
  "public_ip": "203.0.113.10"
}
```

**Fields**:
- `cluster_id` (string, required): UUID of parent cluster
- `name` (string, required): Unique node name within cluster
- `is_lighthouse` (boolean, required): Whether node is a lighthouse
- `overlay_ip` (string, required): Nebula overlay IP (CIDR notation)
- `public_ip` (string, optional): Public IP for lighthouse nodes

**Response**: 201 Created

```json
{
  "id": "node-uuid",
  "cluster_id": "cluster-uuid",
  "name": "node-001",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.5",
  "public_ip": "",
  "auth_token": "abc123def456789...",
  "created_at": "2025-11-22T10:30:45Z",
  "updated_at": "2025-11-22T10:30:45Z"
}
```

**Note**: The `auth_token` is only returned on creation. Store it securely!

**Example**:

```bash
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin-token" \
  -d '{
    "cluster_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "node-001",
    "is_lighthouse": false,
    "overlay_ip": "10.42.0.5"
  }'
```

**Validation**:
- Name must be unique within cluster
- Overlay IP must be valid CIDR
- Public IP required for lighthouse nodes
- Cluster must exist

### GET /api/v1/nodes

List nodes, optionally filtered by cluster.

**Authentication**: Required

**Query Parameters**:
- `cluster_id` (string, optional): Filter by cluster UUID

**Response**: 200 OK

```json
{
  "nodes": [
    {
      "id": "node-uuid-1",
      "cluster_id": "cluster-uuid",
      "name": "node-001",
      "is_lighthouse": true,
      "overlay_ip": "10.42.0.1",
      "public_ip": "203.0.113.10",
      "last_seen_at": "2025-11-22T10:30:45Z",
      "created_at": "2025-11-22T10:00:00Z",
      "updated_at": "2025-11-22T10:30:45Z"
    },
    {
      "id": "node-uuid-2",
      "cluster_id": "cluster-uuid",
      "name": "node-002",
      "is_lighthouse": false,
      "overlay_ip": "10.42.0.5",
      "public_ip": "",
      "last_seen_at": null,
      "created_at": "2025-11-22T10:15:00Z",
      "updated_at": "2025-11-22T10:15:00Z"
    }
  ],
  "total": 2
}
```

**Example**:

```bash
# List all nodes
curl http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer admin-token"

# List nodes in specific cluster
curl http://localhost:8080/api/v1/nodes?cluster_id=550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer admin-token"
```

### GET /api/v1/nodes/:id

Get details of a specific node.

**Authentication**: Required

**Path Parameters**:
- `id` (string): Node UUID

**Response**: 200 OK

```json
{
  "id": "node-uuid",
  "cluster_id": "cluster-uuid",
  "name": "node-001",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.5",
  "public_ip": "",
  "last_seen_at": "2025-11-22T10:30:45Z",
  "created_at": "2025-11-22T10:00:00Z",
  "updated_at": "2025-11-22T10:30:45Z"
}
```

**Example**:

```bash
curl http://localhost:8080/api/v1/nodes/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer node-token"
```

### PUT /api/v1/nodes/:id

Update a node.

**Authentication**: Required

**Path Parameters**:
- `id` (string): Node UUID

**Request Body**:

```json
{
  "name": "node-001-updated",
  "public_ip": "203.0.113.20"
}
```

**Updatable Fields**:
- `name` (string, optional): New node name
- `public_ip` (string, optional): New public IP
- `overlay_ip` (string, optional): New overlay IP

**Response**: 200 OK

```json
{
  "id": "node-uuid",
  "cluster_id": "cluster-uuid",
  "name": "node-001-updated",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.5",
  "public_ip": "203.0.113.20",
  "last_seen_at": "2025-11-22T10:30:45Z",
  "created_at": "2025-11-22T10:00:00Z",
  "updated_at": "2025-11-22T10:35:00Z"
}
```

**Example**:

```bash
curl -X PUT http://localhost:8080/api/v1/nodes/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin-token" \
  -d '{
    "public_ip": "203.0.113.20"
  }'
```

### DELETE /api/v1/nodes/:id

Delete a node.

**Authentication**: Required

**Path Parameters**:
- `id` (string): Node UUID

**Response**: 204 No Content

**Example**:

```bash
curl -X DELETE http://localhost:8080/api/v1/nodes/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer admin-token"
```

**Consequences**:
- Node auth token invalidated
- Node removed from topology
- Lighthouse process stopped (if applicable)

### POST /api/v1/nodes/:id/token

Regenerate authentication token for a node.

**Authentication**: Required (admin)

**Path Parameters**:
- `id` (string): Node UUID

**Response**: 200 OK

```json
{
  "id": "node-uuid",
  "auth_token": "new-token-abc123def456...",
  "message": "Token regenerated successfully. Save this token - it won't be shown again."
}
```

**Example**:

```bash
curl -X POST http://localhost:8080/api/v1/nodes/550e8400-e29b-41d4-a716-446655440000/token \
  -H "Authorization: Bearer admin-token"
```

**Note**: Old token immediately invalidated. Update node daemon with new token.

## Config Bundle Management

### POST /api/v1/bundles/:cluster_id

Upload a configuration bundle for a cluster.

**Authentication**: Required (admin)

**Path Parameters**:
- `cluster_id` (string): Cluster UUID

**Request Headers**:
```http
Content-Type: application/octet-stream
X-Bundle-Version: 1.0.0
```

**Request Body**: Binary tarball containing:
- `ca.crt` (required): Nebula CA certificate
- `config.yml` (required): Nebula configuration template
- `nebula-lighthouse` (optional): Lighthouse binary for master
- `nebula-cert` (optional): Certificate utility

**Response**: 201 Created

```json
{
  "id": "bundle-uuid",
  "cluster_id": "cluster-uuid",
  "version": "1.0.0",
  "hash": "sha256:abc123...",
  "size_bytes": 1048576,
  "created_at": "2025-11-22T10:30:45Z"
}
```

**Example**:

```bash
# Create bundle tarball
tar czf bundle.tar.gz ca.crt config.yml nebula-lighthouse

# Upload bundle
curl -X POST http://localhost:8080/api/v1/bundles/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer admin-token" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Bundle-Version: 1.0.0" \
  --data-binary @bundle.tar.gz
```

**Validation**:
- Tarball must be valid gzip
- Must contain ca.crt and config.yml
- Version must follow semver (e.g., 1.0.0)
- Size must be under 100MB (configurable)

### GET /api/v1/bundles/:cluster_id

List config bundles for a cluster.

**Authentication**: Required

**Path Parameters**:
- `cluster_id` (string): Cluster UUID

**Response**: 200 OK

```json
{
  "bundles": [
    {
      "id": "bundle-uuid-1",
      "cluster_id": "cluster-uuid",
      "version": "1.1.0",
      "hash": "sha256:def456...",
      "size_bytes": 1048576,
      "created_at": "2025-11-22T12:00:00Z"
    },
    {
      "id": "bundle-uuid-2",
      "cluster_id": "cluster-uuid",
      "version": "1.0.0",
      "hash": "sha256:abc123...",
      "size_bytes": 1048576,
      "created_at": "2025-11-22T10:30:45Z"
    }
  ],
  "total": 2
}
```

**Example**:

```bash
curl http://localhost:8080/api/v1/bundles/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer node-token"
```

### GET /api/v1/bundles/:cluster_id/:version

Download a specific config bundle version.

**Authentication**: Required

**Path Parameters**:
- `cluster_id` (string): Cluster UUID
- `version` (string): Bundle version (or "latest")

**Response**: 200 OK

**Headers**:
```http
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="bundle-1.0.0.tar.gz"
X-Bundle-Hash: sha256:abc123...
X-Bundle-Version: 1.0.0
```

**Body**: Binary tarball

**Example**:

```bash
# Download specific version
curl -o bundle.tar.gz \
  http://localhost:8080/api/v1/bundles/550e8400-e29b-41d4-a716-446655440000/1.0.0 \
  -H "Authorization: Bearer node-token"

# Download latest version
curl -o bundle.tar.gz \
  http://localhost:8080/api/v1/bundles/550e8400-e29b-41d4-a716-446655440000/latest \
  -H "Authorization: Bearer node-token"

# Verify hash
sha256sum bundle.tar.gz
```

### DELETE /api/v1/bundles/:cluster_id/:version

Delete a config bundle version.

**Authentication**: Required (admin)

**Path Parameters**:
- `cluster_id` (string): Cluster UUID
- `version` (string): Bundle version

**Response**: 204 No Content

**Example**:

```bash
curl -X DELETE \
  http://localhost:8080/api/v1/bundles/550e8400-e29b-41d4-a716-446655440000/1.0.0 \
  -H "Authorization: Bearer admin-token"
```

**Note**: Cannot delete currently active bundle for lighthouses.

## Topology Management

### GET /api/v1/topology/:cluster_id

Get network topology for a cluster.

**Authentication**: Required

**Path Parameters**:
- `cluster_id` (string): Cluster UUID

**Response**: 200 OK

```json
{
  "cluster_id": "cluster-uuid",
  "nodes": [
    {
      "node_id": "node-uuid-1",
      "name": "lighthouse-001",
      "is_lighthouse": true,
      "overlay_ip": "10.42.0.1",
      "public_ip": "203.0.113.10",
      "port": 4242
    },
    {
      "node_id": "node-uuid-2",
      "name": "node-001",
      "is_lighthouse": false,
      "overlay_ip": "10.42.0.5",
      "public_ip": "",
      "port": 0
    }
  ],
  "lighthouses": [
    {
      "node_id": "node-uuid-1",
      "overlay_ip": "10.42.0.1",
      "public_ip": "203.0.113.10",
      "port": 4242
    }
  ],
  "updated_at": "2025-11-22T10:30:45Z"
}
```

**Example**:

```bash
curl http://localhost:8080/api/v1/topology/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer node-token"
```

### POST /api/v1/topology/:cluster_id

Update network topology for a cluster.

**Authentication**: Required (admin)

**Path Parameters**:
- `cluster_id` (string): Cluster UUID

**Request Body**:

```json
{
  "nodes": [
    {
      "node_id": "node-uuid-1",
      "is_lighthouse": true,
      "overlay_ip": "10.42.0.1",
      "public_ip": "203.0.113.10",
      "port": 4242
    },
    {
      "node_id": "node-uuid-2",
      "is_lighthouse": false,
      "overlay_ip": "10.42.0.5",
      "public_ip": "",
      "port": 0
    }
  ]
}
```

**Response**: 200 OK

```json
{
  "message": "Topology updated successfully",
  "cluster_id": "cluster-uuid",
  "nodes_updated": 2,
  "lighthouses": 1
}
```

**Example**:

```bash
curl -X POST http://localhost:8080/api/v1/topology/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer admin-token" \
  -d '{
    "nodes": [
      {
        "node_id": "550e8400-e29b-41d4-a716-446655440001",
        "is_lighthouse": true,
        "overlay_ip": "10.42.0.1",
        "public_ip": "203.0.113.10",
        "port": 4242
      }
    ]
  }'
```

**Validation**:
- All node IDs must exist in cluster
- Lighthouse nodes must have public IP and port
- Overlay IPs must be unique within cluster
- At least one lighthouse required

## Rate Limiting

NebulaGC implements multi-level rate limiting to protect against abuse.

### Rate Limit Headers

All responses include rate limit headers:

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1700654400
```

### Rate Limit Tiers

1. **Global Limit**: 1000 requests/second (all clients)
2. **Per-IP Limit**: 100 requests/minute (per source IP)
3. **Per-Node Limit**: 20 requests/minute (per authenticated node)
4. **Per-Cluster Limit**: 200 requests/minute (per cluster)

### Rate Limit Exceeded Response

**Status**: 429 Too Many Requests

```json
{
  "error": "Rate limit exceeded",
  "code": "RATE_LIMIT_EXCEEDED",
  "details": {
    "limit": 100,
    "reset_at": "2025-11-22T10:35:00Z"
  },
  "request_id": "correlation-id"
}
```

### Retry Strategy

```bash
# Check Retry-After header
curl -i http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer node-token"

# Response includes:
# HTTP/1.1 429 Too Many Requests
# Retry-After: 60

# Wait specified seconds before retry
sleep 60
curl http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer node-token"
```

## Error Codes

### Authentication Errors

- `AUTH_MISSING` - Authorization header missing
- `AUTH_INVALID` - Invalid token format or value
- `AUTH_EXPIRED` - Token has expired (future feature)
- `AUTH_INSUFFICIENT` - Insufficient permissions for operation

### Validation Errors

- `VALIDATION_FAILED` - Request validation failed
- `INVALID_JSON` - Malformed JSON in request body
- `INVALID_FIELD` - Invalid value for specific field
- `MISSING_FIELD` - Required field missing
- `DUPLICATE_NAME` - Resource name already exists

### Resource Errors

- `NOT_FOUND` - Resource not found
- `ALREADY_EXISTS` - Resource already exists
- `CONFLICT` - Operation conflicts with current state
- `REFERENCED` - Cannot delete (referenced by other resources)

### System Errors

- `DATABASE_ERROR` - Database operation failed
- `INTERNAL_ERROR` - Unexpected internal error
- `SERVICE_UNAVAILABLE` - Service temporarily unavailable
- `RATE_LIMIT_EXCEEDED` - Too many requests

### Example Error Handling

```bash
# Capture full error response
response=$(curl -s -w "\n%{http_code}" http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer invalid-token")

http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "401" ]; then
  error_code=$(echo "$body" | jq -r '.code')
  if [ "$error_code" = "AUTH_INVALID" ]; then
    echo "Token is invalid. Please regenerate."
  fi
fi
```

## Pagination (Future)

Currently, all list endpoints return all results. Future versions will support pagination:

```bash
# Future API (not yet implemented)
curl "http://localhost:8080/api/v1/nodes?page=1&per_page=50" \
  -H "Authorization: Bearer node-token"
```

Response will include pagination metadata:

```json
{
  "nodes": [...],
  "total": 500,
  "page": 1,
  "per_page": 50,
  "total_pages": 10
}
```

## Webhooks (Future)

Future versions will support webhooks for event notifications:

```json
{
  "event": "node.created",
  "timestamp": "2025-11-22T10:30:45Z",
  "data": {
    "node_id": "node-uuid",
    "cluster_id": "cluster-uuid"
  }
}
```

## API Versioning

Current version: **v1**

### Version Support Policy

- **v1**: Stable, supported for all 1.x releases
- **Breaking changes**: Introduce new version (v2) alongside v1
- **Deprecation**: Minimum 6 months notice before removing old version
- **Migration**: Migration guides provided for breaking changes

### Version Detection

```bash
# Check API version from /version endpoint
curl http://localhost:8080/version | jq '.version'

# API path includes version
# /api/v1/... for version 1
# /api/v2/... for version 2 (future)
```

## SDK Support

Official SDKs available:

- **Go SDK**: `github.com/yaroslav-gwit/nebulagc/sdk`
- **Python SDK**: Coming soon
- **JavaScript SDK**: Coming soon

### Go SDK Example

```go
import "github.com/yaroslav-gwit/nebulagc/sdk"

// Create client
client, err := sdk.NewClient(&sdk.Config{
    BaseURL:   "http://localhost:8080",
    AuthToken: "node-token",
})

// Create node
node, err := client.CreateNode(ctx, &sdk.CreateNodeRequest{
    ClusterID:    "cluster-uuid",
    Name:         "node-001",
    IsLighthouse: false,
    OverlayIP:    "10.42.0.5",
})

// List nodes
nodes, err := client.ListNodes(ctx, &sdk.ListNodesRequest{
    ClusterID: "cluster-uuid",
})
```

## Support

- **Documentation**: https://github.com/yaroslav-gwit/nebulagc/docs
- **Issues**: https://github.com/yaroslav-gwit/nebulagc/issues
- **Discussions**: https://github.com/yaroslav-gwit/nebulagc/discussions

## Changelog

See [CHANGELOG.md](../CHANGELOG.md) for API changes and version history.
