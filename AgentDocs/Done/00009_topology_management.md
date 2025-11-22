# Task 00009: Topology Management

**Status**: Completed
**Started**: 2025-01-21
**Completed**: 2025-01-21
**Phase**: 1 (HA Control Plane Core)
**Dependencies**: Task 00007 (Node Management)

## Objective

Implement route registration, lighthouse/relay assignment, and cluster topology queries for managing Nebula network topology.

## Implementation Summary

Successfully implemented a complete topology management system including:
- Route registration with CIDR validation
- Lighthouse assignment with public IP and port
- Relay assignment for NAT traversal
- Complete topology queries
- Cluster token rotation
- All operations bump config version for node synchronization

## Files Created

### Service Layer
- **server/internal/service/topology.go** (570 lines)
  - TopologyService for business logic
  - UpdateRoutes() with CIDR validation
  - GetNodeRoutes() and GetClusterRoutes()
  - SetLighthouse() and UnsetLighthouse()
  - SetRelay() and UnsetRelay()
  - GetTopology() returning complete cluster topology
  - RotateClusterToken() for cluster token rotation
  - All operations use transactions and bump config version

- **server/internal/service/topology_test.go** (403 lines)
  - 12 comprehensive test cases
  - Tests for route management (update, clear, get cluster routes)
  - Tests for lighthouse management (set, unset, multiple)
  - Tests for relay management (set, unset, multiple)
  - Tests for topology queries
  - Tests for token rotation
  - All tests passing with in-memory SQLite

### HTTP Layer
- **server/internal/api/handlers/topology.go** (337 lines)
  - TopologyHandler with 9 endpoints
  - UpdateRoutes() - Any node can update routes (PUT)
  - GetRoutes() - Get authenticated node's routes (GET)
  - GetClusterRoutes() - Get all cluster routes (GET)
  - AssignLighthouse() - Set lighthouse status (POST, cluster token)
  - UnassignLighthouse() - Remove lighthouse status (DELETE, cluster token)
  - AssignRelay() - Set relay status (POST, cluster token)
  - UnassignRelay() - Remove relay status (DELETE, cluster token)
  - GetTopology() - Get complete topology (GET, cluster token)
  - RotateClusterToken() - Rotate cluster token (POST, cluster token)

### Integration
- **server/internal/api/router.go** (modified)
  - Added topology service and handler initialization
  - Wired up 3 route endpoints (node token auth)
  - Wired up 5 topology endpoints (cluster token auth)
  - Wired up 1 token rotation endpoint (cluster token auth)
  - Applied rate limiting (20 req/s for routes, 100 req/s for topology)

## API Endpoints Implemented

### Route Management (Node Token Auth)

#### PUT /api/v1/routes
Updates routes for authenticated node.
- **Auth**: Node token required
- **Body**: `{"routes": ["10.0.1.0/24", "10.0.2.0/24"]}`
- **Response**: `{"message": "Routes updated successfully"}`

#### GET /api/v1/routes
Returns authenticated node's routes.
- **Auth**: Node token required
- **Response**: `{"data": {"routes": ["10.0.1.0/24"]}}`

#### GET /api/v1/routes/cluster
Returns all routes in cluster.
- **Auth**: Node token required
- **Response**: `{"data": {"routes": {"node-id": ["10.0.1.0/24"]}}}`

### Topology Management (Cluster Token Auth)

#### POST /api/v1/topology/lighthouse
Assigns lighthouse status.
- **Auth**: Cluster token required
- **Body**: `{"node_id": "uuid", "public_ip": "203.0.113.1", "port": 4242}`
- **Response**: `{"message": "Lighthouse status assigned"}`

#### DELETE /api/v1/topology/lighthouse/:node_id
Removes lighthouse status.
- **Auth**: Cluster token required
- **Response**: `{"message": "Lighthouse status removed"}`

#### POST /api/v1/topology/relay
Assigns relay status.
- **Auth**: Cluster token required
- **Body**: `{"node_id": "uuid"}`
- **Response**: `{"message": "Relay status assigned"}`

#### DELETE /api/v1/topology/relay/:node_id
Removes relay status.
- **Auth**: Cluster token required
- **Response**: `{"message": "Relay status removed"}`

#### GET /api/v1/topology
Returns complete cluster topology.
- **Auth**: Cluster token required
- **Response**:
```json
{
  "data": {
    "lighthouses": [
      {
        "node_id": "uuid",
        "name": "lighthouse-1",
        "public_ip": "203.0.113.1",
        "port": 4242
      }
    ],
    "relays": [
      {
        "node_id": "uuid",
        "name": "relay-1"
      }
    ],
    "routes": {
      "node-id-1": ["10.0.1.0/24"]
    }
  }
}
```

### Token Rotation (Cluster Token Auth)

#### POST /api/v1/tokens/cluster/rotate
Rotates cluster token.
- **Auth**: Cluster token required
- **Response**: `{"data": {"token": "new-token", "message": "Cluster token rotated successfully"}}`

## Route Management

Routes allow nodes to advertise internal networks they can reach.

### Validation
- CIDR notation (e.g., "10.0.0.0/24")
- Valid IP address and subnet mask using net.ParseCIDR()
- Empty array clears all routes

### Storage
- Stored as JSON array in `nodes.routes` column
- Updates tracked in `nodes.routes_updated_at` timestamp
- Config version bumped on changes

## Lighthouse Management

Lighthouses are nodes with static IPs that help other nodes discover each other.

### Features
- Multiple lighthouses per cluster supported
- Public IP and port required
- Port can be 0 to use cluster default
- Setting/unsetting bumps config version

### Database Fields
- `nodes.is_lighthouse` (0 or 1)
- `nodes.lighthouse_public_ip` (TEXT)
- `nodes.lighthouse_port` (INTEGER)
- `nodes.lighthouse_relay_updated_at` (timestamp)

## Relay Management

Relays allow nodes behind restrictive NAT to communicate through intermediate nodes.

### Features
- Multiple relays per cluster supported
- No additional configuration required
- Setting/unsetting bumps config version

### Database Fields
- `nodes.is_relay` (0 or 1)
- `nodes.lighthouse_relay_updated_at` (timestamp)

## Cluster Token Rotation

### Security Features
- Generates new 41+ character token
- HMAC-SHA256 hashing with server secret
- Immediate invalidation of old token
- Returns plaintext token only once

### Flow
1. Generate new token with pkg/token.Generate()
2. Hash with HMAC-SHA256
3. Update `clusters.cluster_token_hash`
4. Return plaintext token to caller
5. Old token immediately invalid

## Testing Results

### Topology Service Tests
```
TestTopologyService_UpdateRoutes              PASS
TestTopologyService_UpdateRoutesInvalidCIDR   PASS
TestTopologyService_UpdateRoutesClearAll      PASS
TestTopologyService_GetClusterRoutes          PASS
TestTopologyService_SetLighthouse             PASS
TestTopologyService_UnsetLighthouse           PASS
TestTopologyService_SetRelay                  PASS
TestTopologyService_UnsetRelay                PASS
TestTopologyService_GetTopology               PASS
TestTopologyService_RotateClusterToken        PASS
TestTopologyService_MultipleLighthouses       PASS
TestTopologyService_MultipleRelays            PASS
```

### Build Status
- ✅ Server builds successfully
- ✅ All imports resolved
- ✅ No import cycles
- ✅ All tests passing (12 new tests + existing tests)

## Security Features

1. **CIDR Validation**: Uses net.ParseCIDR() for proper validation
2. **IP Validation**: Uses net.ParseIP() for lighthouse IPs
3. **Authentication Layers**:
   - Node token auth for route operations (nodes manage their own routes)
   - Cluster token auth for topology operations (admin-level changes)
4. **Rate Limiting**:
   - 20 req/s per node for route updates
   - 100 req/s per cluster for topology operations
5. **Generic Errors**: HTTP responses use generic messages to prevent info disclosure
6. **Config Version Bumping**: All changes trigger node updates

## Standards Compliance

- ✅ All functions have documentation comments
- ✅ All structs and fields documented
- ✅ CIDR validation for routes
- ✅ IP validation for lighthouses
- ✅ Cluster-token authentication for topology changes
- ✅ Node-token authentication for route updates
- ✅ Config version bumping on all changes
- ✅ Transaction safety for atomic updates
- ✅ Comprehensive test coverage
- ✅ No code duplication

## Database Schema Used

### nodes Table Fields
```sql
routes TEXT,                             -- JSON array of CIDR strings
routes_updated_at DATETIME,              -- Route update timestamp
is_lighthouse INTEGER,                   -- Boolean (0 or 1)
lighthouse_public_ip TEXT,               -- Public IP for lighthouse
lighthouse_port INTEGER,                 -- UDP port
is_relay INTEGER,                        -- Boolean (0 or 1)
lighthouse_relay_updated_at DATETIME,    -- Status change timestamp
```

### clusters Table Fields
```sql
config_version INTEGER,        -- Incremented on topology changes
cluster_token_hash TEXT,       -- HMAC-SHA256 hash
```

## Integration Points

### Upstream Dependencies
- Authentication middleware (node and cluster token validation)
- Cluster/node context helpers (getClusterID, getNodeID)
- Database connection
- Logger (Zap)
- Token package (generation and hashing)

### Downstream Consumers
- Task 00011: Lighthouse process manager will use topology data
- Client SDK: Will query topology for routing decisions
- Node daemon: Will poll for topology changes

## Performance Considerations

1. **Transaction Safety**: All updates use transactions for atomicity
2. **Config Version Bumping**: Atomic increment prevents race conditions
3. **JSON Encoding**: Routes stored as JSON for efficient querying
4. **Indexed Queries**: Topology queries use cluster_id index
5. **Rate Limiting**: Prevents abuse of topology endpoints

## Next Steps

The following tasks depend on this implementation:
- **Task 00010**: Server CLI with Cobra (admin operations)
- **Task 00011**: Lighthouse process management (uses topology data)

## Notes

Topology management is central to Nebula operation:
- **Routes** enable efficient packet forwarding to internal networks
- **Lighthouses** enable peer discovery in the mesh
- **Relays** enable communication through restrictive NATs
- All topology changes bump config version, triggering node updates
- Multiple lighthouses and relays provide redundancy

The separation of auth levels (node token for routes, cluster token for topology) ensures nodes can manage their own routes while preventing unauthorized topology changes.
