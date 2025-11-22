# Task 00015: SDK Topology Methods

**Status**: Completed
**Started**: 2025-11-21
**Completed**: 2025-11-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00012 (SDK Foundation)

## Objective

Implement SDK methods for routes, lighthouses, relays, and cluster topology management.

## Implementation Plan

### Files to Modify

1. **sdk/client.go** - Add topology management methods
2. **sdk/client_test.go** - Add test cases for topology methods

### Methods to Implement

```go
func (c *Client) RegisterRoutes(ctx context.Context, routes []string) error
func (c *Client) GetRoutes(ctx context.Context) ([]string, error)
func (c *Client) ListClusterRoutes(ctx context.Context) ([]NodeRoutes, error)
func (c *Client) SetLighthouse(ctx context.Context, nodeID string, enabled bool, publicIP string, port int) error
func (c *Client) SetRelay(ctx context.Context, nodeID string, enabled bool) error
func (c *Client) GetTopology(ctx context.Context) (*ClusterTopology, error)
func (c *Client) RotateClusterToken(ctx context.Context) (string, error)
```

## Changes Made

### sdk/client.go
- Added RegisterRoutes() - POST routes to control plane (node token)
- Added GetRoutes() - GET node's registered routes (node token)
- Added ListClusterRoutes() - GET all cluster routes (node token)
- Added SetLighthouse() - PUT lighthouse configuration (node token)
- Added SetRelay() - PUT relay configuration (node token)
- Added GetTopology() - GET cluster topology (node token)
- Added RotateClusterToken() - POST cluster token rotation (cluster token)

### sdk/client_test.go
- Added TestClient_RegisterRoutes (4 test cases)
- Added TestClient_GetRoutes (3 test cases)
- Added TestClient_ListClusterRoutes (3 test cases)
- Added TestClient_SetLighthouse (5 test cases)
- Added TestClient_SetRelay (4 test cases)
- Added TestClient_GetTopology (3 test cases)
- Added TestClient_RotateClusterToken (2 test cases)

**Test Results**: All 85 subtests passed, 79.1% overall coverage

## Testing

✅ Route registration works
✅ Route retrieval returns correct routes
✅ Cluster routes listing works
✅ Lighthouse assignment succeeds
✅ Relay assignment succeeds
✅ Topology query returns all lighthouses and relays
✅ Cluster token rotation returns new token
✅ Authentication headers set correctly
✅ Error cases handled properly (401, 404, 400)

## Rollback Plan

- Revert sdk/client.go changes
- Revert sdk/client_test.go changes
- No database or migration changes in this task

## Notes

- Node route operations use node token authentication
- Cluster-wide operations use cluster token authentication
- Most operations prefer master (write operations)
- Topology query can use any replica (read operation)
