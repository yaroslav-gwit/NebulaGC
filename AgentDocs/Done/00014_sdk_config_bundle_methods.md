# Task 00014: SDK Config Bundle Methods

**Status**: Completed
**Started**: 2025-11-21
**Completed**: 2025-11-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00012 (SDK Foundation)

## Objective

Implement SDK methods for config bundle operations including version checking, bundle download with 304 Not Modified support, and bundle upload.

## Implementation Summary

Successfully implemented all three config bundle methods with comprehensive 304 Not Modified handling and binary data support.

### Files Modified

1. **sdk/client.go** - Added 3 bundle management methods plus helper functions (267 lines)
2. **sdk/client_test.go** - Added 3 test functions with 13 subtests (322 lines)

### Methods Implemented

```go
func (c *Client) GetLatestVersion(ctx context.Context) (int64, error)
func (c *Client) DownloadBundle(ctx context.Context, currentVersion int64) ([]byte, int64, error)
func (c *Client) UploadBundle(ctx context.Context, data []byte) (int64, error)
```

## Changes Made

### sdk/client.go

- ✅ Added `GetLatestVersion()` method
  - GET to `/api/v1/tenants/{tenant}/clusters/{cluster}/config/version`
  - Returns current config bundle version as int64
  - Uses node token authentication
  - Can execute on any instance (preferMaster=false)

- ✅ Added `DownloadBundle()` method with 304 Not Modified support
  - GET to `/api/v1/tenants/{tenant}/clusters/{cluster}/config/bundle?current_version={version}`
  - Returns `(nil, currentVersion, nil)` when server returns 304 Not Modified
  - Returns `(data, newVersion, nil)` when new version available
  - Reads binary tar.gz data from response body
  - Parses version from `X-Config-Version` header
  - Uses node token authentication
  - Can execute on any instance (preferMaster=false)

- ✅ Added `UploadBundle()` method
  - POST to `/api/v1/tenants/{tenant}/clusters/{cluster}/config/bundle`
  - Uploads binary tar.gz data as `application/octet-stream`
  - Returns new version number from response
  - Uses node token authentication
  - Executes on master instance (preferMaster=true)
  - Restricted to admin nodes (server enforces)

- ✅ Added helper functions:
  - `parseVersion()` - Parses version string to int64
  - `parseInt64()` - Generic string to int64 parser

### sdk/client_test.go

- ✅ Added `TestClient_GetLatestVersion` with 4 test cases
  - Successful version retrieval
  - Version zero handling
  - Unauthorized (invalid node token)
  - Invalid response format

- ✅ Added `TestClient_DownloadBundle` with 4 test cases
  - Successful download with new version
  - 304 Not Modified (no update available)
  - Unauthorized (invalid node token)
  - Rate limited

- ✅ Added `TestClient_UploadBundle` with 5 test cases
  - Successful upload (201 Created)
  - Successful upload (200 OK)
  - Unauthorized - not admin
  - Invalid bundle format
  - Rate limited

## Testing Results

All tests passing with good coverage:

```
=== RUN   TestClient_GetLatestVersion (4 subtests) - PASS
=== RUN   TestClient_DownloadBundle (4 subtests) - PASS
=== RUN   TestClient_UploadBundle (5 subtests) - PASS

Total: 19 test functions, 64 subtests, 0 failures
Coverage: 76.3% of statements
```

## Key Implementation Details

1. **Authentication**: All methods use `AuthTypeNode` for node token authentication
2. **Master Preference**: 
   - GetLatestVersion and DownloadBundle use `preferMaster=false` (read operations)
   - UploadBundle uses `preferMaster=true` (write operation)
3. **304 Not Modified**: DownloadBundle correctly handles 304 by returning nil data with current version
4. **Binary Data**: DownloadBundle and UploadBundle handle binary tar.gz data correctly
5. **Version Parsing**: Custom parser for version header (X-Config-Version)
6. **Error Handling**: Comprehensive error wrapping and typed errors
7. **Manual Request Handling**: DownloadBundle and UploadBundle use manual request building instead of doJSONRequest for binary data support

## Rollback Plan

- Revert sdk/client.go to remove bundle methods and helpers
- Revert sdk/client_test.go to remove test cases
- No database or migration changes in this task

## Notes

- Methods ready for use by daemon poller (Task 00019)
- 304 Not Modified support critical for efficient polling
- Binary data handling tested with mock tar.gz content
- Admin-only UploadBundle enforced by server, not client
- Helper functions (parseVersion, parseInt64) are internal utilities
