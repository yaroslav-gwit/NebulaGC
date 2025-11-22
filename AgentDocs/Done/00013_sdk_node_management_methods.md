# Task 00013: SDK Node Management Methods

**Status**: Completed
**Started**: 2025-11-21
**Completed**: 2025-11-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00012 (SDK Foundation)

## Objective

Implement SDK methods for node lifecycle operations including creation, deletion, listing, MTU updates, and token rotation.

## Implementation Summary

Successfully implemented all five node management methods in the SDK with comprehensive test coverage (81.1%).

### Files Modified

1. **sdk/client.go** - Added 5 node management methods (157 lines)
2. **sdk/client_test.go** - Added 5 test functions with 18 subtests (408 lines)

### Methods Implemented

```go
func (c *Client) CreateNode(ctx context.Context, name string, isAdmin bool, mtu int) (*NodeCredentials, error)
func (c *Client) DeleteNode(ctx context.Context, nodeID string) error
func (c *Client) ListNodes(ctx context.Context, page, pageSize int) ([]NodeSummary, error)
func (c *Client) UpdateMTU(ctx context.Context, nodeID string, mtu int) error
func (c *Client) RotateNodeToken(ctx context.Context, nodeID string) (string, error)
```

## Changes Made

### sdk/client.go
- ✅ Added `CreateNode()` method with full documentation
  - POST to `/api/v1/tenants/{tenant}/clusters/{cluster}/nodes`
  - Returns NodeCredentials with ID, token, and Nebula IP
  - Uses cluster token authentication
  - Executes on master instance (preferMaster=true)
  
- ✅ Added `DeleteNode()` method with full documentation
  - DELETE to `/api/v1/tenants/{tenant}/clusters/{cluster}/nodes/{node}`
  - Uses cluster token authentication
  - Executes on master instance (preferMaster=true)
  
- ✅ Added `ListNodes()` method with full documentation
  - GET to `/api/v1/tenants/{tenant}/clusters/{cluster}/nodes`
  - Supports pagination (page, pageSize)
  - Returns slice of NodeSummary
  - Uses cluster token authentication
  - Can execute on any instance (preferMaster=false)
  
- ✅ Added `UpdateMTU()` method with full documentation
  - PUT to `/api/v1/tenants/{tenant}/clusters/{cluster}/nodes/{node}/mtu`
  - Validates MTU range (576-9000)
  - Uses cluster token authentication
  - Executes on master instance (preferMaster=true)
  
- ✅ Added `RotateNodeToken()` method with full documentation
  - POST to `/api/v1/tenants/{tenant}/clusters/{cluster}/nodes/{node}/rotate-token`
  - Returns new token string
  - Uses cluster token authentication
  - Executes on master instance (preferMaster=true)

### sdk/client_test.go
- ✅ Added `TestClient_CreateNode` with 4 test cases
  - Successful creation (regular node)
  - Successful creation (admin node)
  - Unauthorized (invalid cluster token)
  - Invalid MTU (validation error)
  
- ✅ Added `TestClient_DeleteNode` with 3 test cases
  - Successful deletion
  - Node not found (404)
  - Unauthorized (invalid cluster token)
  
- ✅ Added `TestClient_ListNodes` with 3 test cases
  - Successful list with nodes
  - Empty list
  - Unauthorized (invalid cluster token)
  
- ✅ Added `TestClient_UpdateMTU` with 5 test cases
  - Successful update
  - Invalid MTU - too low
  - Invalid MTU - too high
  - Node not found (404)
  - Unauthorized (invalid cluster token)
  
- ✅ Added `TestClient_RotateNodeToken` with 3 test cases
  - Successful rotation
  - Node not found (404)
  - Unauthorized (invalid cluster token)

## Testing Results

All tests passing with excellent coverage:

```
=== RUN   TestClient_CreateNode (4 subtests) - PASS
=== RUN   TestClient_DeleteNode (3 subtests) - PASS
=== RUN   TestClient_ListNodes (3 subtests) - PASS
=== RUN   TestClient_UpdateMTU (5 subtests) - PASS
=== RUN   TestClient_RotateNodeToken (3 subtests) - PASS

Total: 16 test functions, 51 subtests, 0 failures
Coverage: 81.1% of statements
```

## Key Implementation Details

1. **Authentication**: All methods use `AuthTypeCluster` for cluster token authentication
2. **Master Preference**: Write operations (create, delete, update, rotate) use `preferMaster=true`
3. **Read Operations**: ListNodes uses `preferMaster=false` for HA load distribution
4. **Error Handling**: Comprehensive error wrapping with context-specific messages
5. **Documentation**: Every method has detailed comments including:
   - Purpose and behavior
   - Authentication requirements
   - Parameters with types and constraints
   - Return values and error conditions
   - Examples of error types (ErrUnauthorized, ErrNotFound, ErrRateLimited)

## Rollback Plan

- Revert sdk/client.go to remove node management methods
- Revert sdk/client_test.go to remove test cases
- No database or migration changes in this task

## Notes

- All methods follow existing SDK patterns from Task 00012
- Test coverage maintained above project standard (>80%)
- Methods ready for use by daemon implementation (Task 00018+)
- Error handling matches server API expectations
- Authentication headers correctly set for all requests
