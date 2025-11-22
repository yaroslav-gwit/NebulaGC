# Task 00012: Go Client SDK Foundation

**Status**: Completed
**Started**: 2025-01-21
**Completed**: 2025-01-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00002 (Shared Models Package)

## Objective

Create the core SDK structure with authentication, request handling, and HA support for programmatic access to the NebulaGC control plane.

## Implementation Summary

Successfully implemented a complete Go client SDK with HA support, automatic master discovery, and comprehensive retry logic.

### Files Created

1. **sdk/errors.go** (38 lines) - SDK-specific error types
2. **sdk/types.go** (147 lines) - Request/response types for API operations
3. **sdk/config.go** (116 lines) - Client configuration with validation and defaults
4. **sdk/auth.go** (48 lines) - Authentication header helpers
5. **sdk/transport.go** (87 lines) - HTTP retry logic with exponential backoff
6. **sdk/client.go** (268 lines) - Main client with HA support and master discovery
7. **sdk/config_test.go** (175 lines) - Configuration tests
8. **sdk/client_test.go** (279 lines) - Client tests with mock servers

### Core Features Implemented

- ✅ Multiple `BaseURLs` for HA support
- ✅ Master discovery via `/api/v1/check-master` endpoint
- ✅ Automatic failover on connection errors
- ✅ Cached master URL with refresh on failure
- ✅ Request retry logic with exponential backoff and jitter
- ✅ Proper header injection for all requests
- ✅ Support for both node token and cluster token authentication
- ✅ Thread-safe master URL caching with sync.RWMutex
- ✅ Configurable timeouts and retry attempts
- ✅ Graceful error handling with typed errors

### Client Interface

```go
type Client struct {
    BaseURLs      []string
    TenantID      string
    ClusterID     string
    NodeID        string
    NodeToken     string
    ClusterToken  string
    HTTPClient    *http.Client
    RetryAttempts int
    RetryWaitMin  time.Duration
    RetryWaitMax  time.Duration
    masterURL     string
    mu            sync.RWMutex
}

func NewClient(config ClientConfig) (*Client, error)
func (c *Client) DiscoverMaster(ctx context.Context) error
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader, authType AuthType, preferMaster bool) (*http.Response, error)
func (c *Client) doJSONRequest(ctx context.Context, method, path string, reqBody, respBody interface{}, authType AuthType, preferMaster bool) error
```

## Changes Made

### sdk/ Package (Complete)

- ✅ **errors.go** - 13 error types for SDK operations
- ✅ **types.go** - Request/response structs (NodeCredentials, NodeSummary, ClusterTopology, etc.)
- ✅ **config.go** - Configuration validation with sensible defaults
- ✅ **auth.go** - AuthType enum and addAuthHeaders() method
- ✅ **transport.go** - Retry logic with exponential backoff and jitter
- ✅ **client.go** - Client struct with HA support, master discovery, and request methods
- ✅ **config_test.go** - 10 test cases for configuration
- ✅ **client_test.go** - 10 test cases for client functionality

### Testing Results

All tests passing:

```text
TestNewClient                         PASS (2 subtests)
TestClient_DiscoverMaster             PASS (3 subtests)
TestClient_ClearMasterCache           PASS
TestClient_BuildURLList               PASS (4 subtests)
TestClient_DoRequest_Authentication   PASS (5 subtests)
TestClient_CalculateBackoff           PASS (4 subtests)
TestClientConfig_Validate             PASS (9 subtests)
TestClientConfig_HasNodeAuth          PASS (3 subtests)
TestClientConfig_HasClusterAuth       PASS (3 subtests)
TestClientConfig_Defaults             PASS

Total: 10 test functions, 33 subtests, 0 failures
```

## Dependencies

- **Task 00002**: Shared models package (for error types and common structures)
- **Task 00005**: REST API must be running for integration tests

## Rollback Plan

- Delete `sdk/` directory
- Remove any imports of `sdk` package from other files
- Revert go.work if sdk module was added

## Notes

- SDK must support both single control plane and HA scenarios
- Should work with or without master discovery (single URL)
- Retry logic should use exponential backoff
- All methods should accept context.Context for cancellation
- Authentication headers must match server expectations:
  - `X-NebulaGC-Node-Token` for node operations
  - `X-NebulaGC-Cluster-Token` for cluster operations
