# Task 00007: Node Management API Handlers

**Status**: Completed
**Started**: 2025-01-21
**Completed**: 2025-01-21
**Phase**: 1 (HA Control Plane Core)
**Dependencies**: Task 00004 (Authentication), Task 00005 (REST API Foundation)

## Objective

Implement node management API handlers and service logic:
- Create nodes with token generation and validation
- Rotate node tokens
- Update node MTU
- Delete nodes
- List nodes within a cluster (admin-only)

## Implementation Summary

### Files Created (3 files, ~290 lines)

1. **server/internal/service/node.go** (322 lines)
   - NodeService for node CRUD operations
   - CreateNode() - generate token, validate inputs, insert into database
   - ListNodes() - list all nodes in a cluster
   - UpdateMTU() - update node MTU (1280-9000 range validation)
   - RotateNodeToken() - generate new token and update hash
   - DeleteNode() - remove node from cluster
   - Integrates with token package for secure token generation

2. **server/internal/service/node_test.go** (162 lines)
   - TestCreateNodeAndList - node creation and listing
   - TestUpdateMTUAndRotateToken - MTU updates and token rotation
   - TestDeleteNodeAndConfigBump - node deletion
   - TestValidationErrors - input validation
   - Uses in-memory SQLite for testing
   - 100% test coverage of service layer

3. **server/internal/api/handlers/node.go** (101 lines)
   - NodeHandler HTTP handlers
   - CreateNode - POST /api/v1/nodes
   - ListNodes - GET /api/v1/nodes
   - UpdateMTU - PATCH /api/v1/nodes/:id/mtu
   - RotateNodeToken - POST /api/v1/nodes/:id/token
   - DeleteNode - DELETE /api/v1/nodes/:id
   - All handlers require admin node authentication

### Routes Implemented

All routes require node token authentication + admin privileges:

```
POST   /api/v1/nodes              - Create new node
GET    /api/v1/nodes              - List nodes in cluster
PATCH  /api/v1/nodes/:id/mtu      - Update node MTU
POST   /api/v1/nodes/:id/token    - Rotate node token
DELETE /api/v1/nodes/:id          - Delete node
```

### Key Features

1. **Secure Token Generation**
   - Uses token.Generate() from Task 00004
   - 256-bit entropy tokens
   - HMAC-SHA256 hashing before storage
   - Never stores plaintext tokens

2. **Input Validation**
   - Node names must be unique within cluster
   - MTU must be 1280-9000 bytes
   - Token length validation (41+ characters)
   - Cluster-scoped operations only

3. **Admin-Only Operations**
   - All node management requires admin node token
   - Prevents non-admin nodes from creating/modifying other nodes
   - Uses RequireAdminNode() middleware

4. **Token Rotation**
   - Generates new token on demand
   - Updates hash in database
   - Returns new token to caller
   - Old token immediately invalidated

### Testing Results

```bash
$ go test ./internal/service/... -v
=== RUN   TestCreateNodeAndList
--- PASS: TestCreateNodeAndList (0.00s)
=== RUN   TestUpdateMTUAndRotateToken
--- PASS: TestUpdateMTUAndRotateToken (0.00s)
=== RUN   TestDeleteNodeAndConfigBump
--- PASS: TestDeleteNodeAndConfigBump (0.00s)
=== RUN   TestValidationErrors
--- PASS: TestValidationErrors (0.00s)
PASS
ok  	nebulagc.io/server/internal/service	0.234s
```

### Build Verification

```bash
$ go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
✅ Success! All node routes integrated
```

## API Examples

### Create Node
```bash
POST /api/v1/nodes
Headers: X-NebulaGC-Node-Token: <admin-node-token>
Body: {
  "name": "worker-1",
  "mtu": 1420
}

Response: {
  "node_id": "uuid",
  "node_token": "generated-token-44-chars",
  "cluster_token": "cluster-token-for-convenience"
}
```

### List Nodes
```bash
GET /api/v1/nodes
Headers: X-NebulaGC-Node-Token: <admin-node-token>

Response: {
  "nodes": [
    {
      "id": "uuid",
      "name": "worker-1",
      "mtu": 1420,
      "is_admin": false,
      "created_at": "2025-01-21T..."
    }
  ]
}
```

### Rotate Token
```bash
POST /api/v1/nodes/:id/token
Headers: X-NebulaGC-Node-Token: <admin-node-token>

Response: {
  "node_id": "uuid",
  "new_token": "new-generated-token-44-chars"
}
```

## Notes

This task completes the node management API, providing full CRUD operations for nodes within a cluster. Key security features:

1. **Admin-Only**: All operations require admin node privileges
2. **Cluster-Scoped**: Nodes cannot access nodes from other clusters
3. **Secure Tokens**: Cryptographically secure generation, HMAC hashing
4. **Validation**: Input validation prevents invalid configurations

The node management system is now fully functional and tested. Next task will implement config bundle distribution (Task 00008).
