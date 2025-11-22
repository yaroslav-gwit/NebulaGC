# Task 00016: SDK Replica Discovery Methods

**Status**: Completed (Partial)
**Started**: 2025-11-21
**Completed**: 2025-11-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00012 (SDK Foundation)

## Objective

Implement SDK methods for replica discovery, master detection, and control plane URL management.

## Implementation Plan

### Files to Modify

1. **sdk/types.go** - Add ReplicaInfo type
2. **sdk/client.go** - Add replica discovery methods
3. **sdk/client_test.go** - Add test cases for replica methods

### Types to Add

```go
// ReplicaInfo contains information about a control plane replica
type ReplicaInfo struct {
    URL      string `json:"url"`
    IsMaster bool   `json:"is_master"`
    Healthy  bool   `json:"healthy"`
}
```

### Methods to Implement

```go
func (c *Client) GetReplicas(ctx context.Context) ([]ReplicaInfo, error)
func (c *Client) CheckMaster(ctx context.Context, url string) (bool, error)
func (c *Client) RefreshControlPlaneList(ctx context.Context) error
```

## Changes Made

### sdk/types.go
- Added MasterStatusResponse type for /health/master endpoint responses

### sdk/client.go
- Added CheckMaster() method to query if a control plane URL is the master instance

### sdk/client_test.go
- Added TestClient_CheckMaster with 5 test cases (master, non-master, service unavailable, invalid JSON, connection failure)

**Test Results**: All 90 subtests passed, 79.7% overall coverage

### Not Implemented (Requires Server-Side API Changes)

The following methods from the task breakdown are not yet implemented because the required server-side API endpoints do not exist:

#### GetReplicas() - List All Replicas
**Missing Endpoint**: `GET /api/v1/replicas` or similar
**Purpose**: Would return a list of all control plane replica instances with their status
**Server Changes Needed**: 
- New API handler in server/internal/api/handlers/
- Route registration in router.go
- Should expose ReplicaService.ListReplicas() via REST API

#### RefreshControlPlaneList() - Update Cached Replica URLs
**Missing Endpoint**: Depends on GetReplicas() endpoint above
**Purpose**: Would query all replicas and update client's BaseURLs cache
**Implementation**: Once GetReplicas() endpoint exists, this method would:
1. Call GetReplicas() to get current replica list
2. Extract URLs from replica info
3. Update client.BaseURLs field
4. Clear cached master URL to force re-discovery

**Recommendation**: Add these endpoints in Phase 3 (Production Hardening) as part of the observability and monitoring enhancements.

## Testing

✅ CheckMaster identifies master instance correctly
✅ CheckMaster identifies non-master instance correctly
✅ CheckMaster handles service unavailable (503)
✅ CheckMaster handles invalid JSON response
✅ CheckMaster handles connection failures
❌ GetReplicas - not implemented (requires server-side API)
❌ RefreshControlPlaneList - not implemented (requires server-side API)
✅ Error handling for unreachable replicas

## Rollback Plan

- Revert sdk/types.go changes
- Revert sdk/client.go changes
- Revert sdk/client_test.go changes
- No database or migration changes in this task

## Notes

- GetReplicas queries control plane for all known replicas
- CheckMaster queries a specific URL's /v1/check-master endpoint
- RefreshControlPlaneList updates the client's BaseURLs with discovered replicas
- All operations are read-only, no authentication required for health checks
