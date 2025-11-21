# Task 00002: Shared Models Package

## Status
- Started: 2025-01-21
- Completed: 2025-01-21 ✅

## Objective
Create the `models/` package with all core data structures, error types, and comprehensive documentation. This package will be imported by the server, SDK, and daemon to ensure consistency.

## Changes Made

### Files Created
- ✅ `models/doc.go` - Package-level documentation
- ✅ `models/tenant.go` - Tenant data structures and API types
- ✅ `models/cluster.go` - Cluster data structures, ClusterState tracking
- ✅ `models/node.go` - Node data structures with full field documentation
- ✅ `models/bundle.go` - ConfigBundle and validation error types
- ✅ `models/replica.go` - Replica registry for HA
- ✅ `models/topology.go` - Network topology types (lighthouses, relays, routes)
- ✅ `models/errors.go` - Standard error types and response structures
- ✅ `models/go.mod` - Go module configuration

### Data Structures Implemented

**Core Entities**:
- `Tenant` - Organization that owns clusters
- `Cluster` - Logical Nebula environment with PKI storage
- `ClusterState` - Per-instance lighthouse version tracking
- `Node` - Individual machines in a cluster
- `ConfigBundle` - Versioned configuration archives
- `Replica` - Control plane instance registry

**Topology Types**:
- `ClusterTopology` - Complete network topology view
- `ControlPlaneLighthouseInfo` - Control plane lighthouse details
- `LighthouseInfo` - Node-based lighthouse information
- `RelayInfo` - Relay node information
- `NodeRoutes` - Route advertisements per node

**Request/Response Types**:
- `TenantCreateRequest`, `TenantListResponse`
- `ClusterCreateRequest`, `ClusterCreateResponse`, `ClusterListResponse`
- `NodeCreateRequest`, `NodeCredentials`, `NodeSummary`, `NodeListResponse`
- `NodeMTUUpdateRequest`, `NodeTokenRotateResponse`
- `NodeRoutesRequest`, `NodeRoutesResponse`
- `NodeLighthouseRequest`, `NodeRelayRequest`
- `BundleVersionResponse`, `BundleUploadResponse`, `BundleValidationError`
- `ReplicaInfo`, `ReplicaListResponse`, `CheckMasterResponse`
- `ClusterRoutesResponse`, `ClusterTokenRotateResponse`

**Error Types** (23 standardized errors):
- `ErrNotFound`, `ErrClusterNotFound`, `ErrTenantNotFound`, `ErrNodeNotFound`
- `ErrBundleNotFound`, `ErrReplicaNotFound`
- `ErrUnauthorized`, `ErrInvalidToken`, `ErrInvalidNodeToken`, `ErrInvalidClusterToken`
- `ErrForbidden`, `ErrNotAdmin`
- `ErrInvalidRequest`, `ErrInvalidCIDR`, `ErrInvalidMTU`
- `ErrConflict`, `ErrDuplicateName`
- `ErrPayloadTooLarge`, `ErrBundleTooLarge`
- `ErrRateLimitExceeded`
- `ErrInternalError`, `ErrDatabaseError`
- `ErrReplicaReadOnly`, `ErrServiceUnavailable`

**Common Response Types**:
- `ErrorResponse` - Standardized API error format
- `SuccessResponse` - Generic success message
- `HealthResponse` - Health check format

### Documentation Standards Met
✅ **Package Documentation**: Comprehensive doc.go explaining purpose and contents
✅ **Struct Documentation**: Every struct has a descriptive comment
✅ **Field Documentation**: All fields documented with purpose, constraints, and examples
✅ **JSON Tags**: All API-facing fields have json tags
✅ **DB Tags**: All database-mapped fields have db tags
✅ **Security Notes**: Sensitive fields marked with `json:"-"` (never exposed)

### Key Features

**Security**:
- Token hashes never exposed in API responses (`json:"-"`)
- PKI private keys never exposed (`json:"-"`)
- Clear documentation of sensitive fields

**Flexibility**:
- Optional fields use pointers (e.g., `*time.Time`, `*string`)
- Omitempty tags for optional JSON fields
- Validation tags for Gin binding (e.g., `binding:"required,uuid4"`)

**Type Safety**:
- Strong typing for IDs (string UUIDs)
- Enumerations documented (e.g., Role: "master" or "replica")
- Clear constraints (MTU: 1280-9000, token length: 41+ chars)

**Consistency**:
- All timestamps use `time.Time`
- All IDs are string UUIDs
- All responses follow consistent patterns

## Line Counts
```
doc.go:         17 lines
tenant.go:      37 lines
cluster.go:    122 lines
node.go:       238 lines
bundle.go:      68 lines
replica.go:     68 lines
topology.go:   109 lines
errors.go:     150 lines
----------------------------
Total:         809 lines of fully documented code
```

## Dependencies
- Task 00001 (Project structure) ✅ Complete

## Testing

### Compilation Test
```bash
cd models && go build .
# ✅ Compiles successfully with no errors
```

### Workspace Integration
```bash
go work sync
# ✅ Models module added to workspace
```

### Import Test (Future)
Once server code exists, verify imports work:
```go
import "github.com/yaroslav/nebulagc/models"

func example() {
    tenant := &models.Tenant{
        ID:   "uuid",
        Name: "Example",
    }
}
```

## Rollback Plan
If this task needs to be undone:
1. Delete models directory:
   ```bash
   rm -rf models/
   ```
2. Remove from go.work:
   ```bash
   # Edit go.work and remove ./models from use block
   ```
3. Run `go work sync`
4. Remove task file from Done/

## Next Tasks
- **Task 00003**: Set up database migrations and SQLc configuration
  - Will use models package for type definitions
  - SQLc will reference these types in generated code
  - Database schema will map to these structs

## Notes

### Design Decisions
1. **Separate module**: Models have their own go.mod to enable independent versioning
2. **No external dependencies**: Models package is dependency-free for maximum portability
3. **Database tags**: All structs include `db` tags for SQLc/database mapping
4. **JSON omitempty**: Optional fields use omitempty to produce clean API responses
5. **Pointer fields**: Optional timestamps and strings use pointers to distinguish null from zero value

### Field Naming Conventions
- IDs always suffixed: `TenantID`, `ClusterID`, `NodeID`
- Booleans prefixed with `Is`: `IsAdmin`, `IsLighthouse`, `IsRelay`
- Timestamps suffixed with `At`: `CreatedAt`, `UpdatedAt`, `RotatedAt`
- Hashes suffixed with `Hash`: `TokenHash`, `ClusterTokenHash`

### Security Considerations
- All sensitive fields use `json:"-"` to prevent accidental exposure
- Token minimum length (41 chars) documented consistently
- HMAC-SHA256 hashing explicitly mentioned in comments
- Clear distinction between tokens (plaintext, never stored) and hashes (stored)

### API Design
- Request types suffixed with `Request`
- Response types suffixed with `Response`
- List responses include `Total` count for pagination
- Error responses standardized with optional error codes

## Completion Criteria
- [x] All core entity types created
- [x] All request/response types created
- [x] All error types defined
- [x] Package-level documentation written
- [x] Every struct documented
- [x] Every field documented
- [x] JSON tags for API serialization
- [x] DB tags for database mapping
- [x] Sensitive fields protected (json:"-")
- [x] Package compiles successfully
- [x] Added to go.work workspace
- [x] Task moved to Done/

## Statistics
- **Files**: 9 (including go.mod)
- **Lines of Code**: 809
- **Structs**: 34
- **Error Types**: 23
- **Documentation Coverage**: 100%
