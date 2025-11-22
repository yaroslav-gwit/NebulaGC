# Task 00008: Config Bundle Management

**Status**: Completed
**Started**: 2025-01-21
**Completed**: 2025-01-21
**Phase**: 1 (HA Control Plane Core)
**Dependencies**: Task 00007 (Node Management)

## Objective

Implement config bundle upload, download, and version management for distributing Nebula configurations to nodes.

## Implementation Summary

Successfully implemented a complete config bundle management system including:
- Bundle format validation (tar.gz with required files)
- Size limits (10 MiB maximum)
- YAML syntax validation
- Version management with incremental versioning
- Conditional HTTP responses (304 Not Modified)
- Storage and retrieval from database

## Files Created

### Core Bundle Package
- **pkg/bundle/types.go** (74 lines)
  - Constants for bundle validation (MaxBundleSize, required file names)
  - Error types (ErrBundleTooLarge, ErrInvalidFormat, ErrMissingRequiredFile, etc.)
  - ValidationResult struct

- **pkg/bundle/validator.go** (~125 lines)
  - Validate() function for comprehensive bundle validation
  - Checks: size, gzip tar format, required files, YAML syntax
  - Returns detailed ValidationResult with files list and size

- **pkg/bundle/validator_test.go** (200 lines)
  - 8 test cases covering all validation scenarios
  - Tests for valid bundles, oversized bundles, invalid formats
  - Tests for missing files, invalid YAML, extra files
  - All tests passing

### Service Layer
- **server/internal/service/bundle.go** (195 lines)
  - BundleService for business logic
  - Upload() with atomic version increment using transactions
  - GetCurrentVersion() for version queries
  - Download() supporting specific version or latest
  - CheckVersion() for conditional response logic

- **server/internal/service/bundle_test.go** (327 lines)
  - 7 comprehensive test cases
  - Tests upload, download, version checking
  - Tests validation error handling
  - Tests multiple uploads and version incrementing
  - All tests passing with in-memory SQLite

### HTTP Layer
- **server/internal/api/handlers/bundle.go** (199 lines)
  - BundleHandler with three endpoints
  - GetVersion() - Returns current config version
  - DownloadBundle() - Supports conditional requests (If-None-Match)
  - UploadBundle() - Validates and stores bundles (admin-only)
  - Proper error mapping for bundle-specific errors

### Integration
- **server/internal/api/router.go** (modified)
  - Added bundle service and handler initialization
  - Wired up three config endpoints with authentication
  - Applied rate limiting (10 req/s for config operations)

- **server/internal/api/handlers/common.go** (modified)
  - Added getClusterID() helper to avoid import cycle

- **go.work** (modified)
  - Added pkg module to workspace

- **pkg/go.mod** (modified)
  - Added gopkg.in/yaml.v3 dependency for YAML validation

## API Endpoints Implemented

### GET /api/v1/config/version
Returns the current config version for the authenticated cluster.
- **Auth**: Node token required
- **Response**: `{"data": {"version": 42}}`

### GET /api/v1/config/bundle
Downloads config bundle with conditional request support.
- **Auth**: Node token required
- **Headers**: If-None-Match: "v123" (optional)
- **Query**: current_version=123 (optional)
- **Response**: 200 with tar.gz data, or 304 Not Modified
- **Headers**: ETag, X-Config-Version, Content-Disposition

### POST /api/v1/config/bundle
Uploads new config bundle.
- **Auth**: Admin node token required
- **Content-Type**: application/gzip
- **Body**: tar.gz archive
- **Response**: `{"data": {"version": 43, "message": "Bundle uploaded successfully"}}`

## Bundle Validation

### Required Files
- `config.yml` - Nebula configuration (must be valid YAML)
- `ca.crt` - CA certificate
- `crl.pem` - Certificate revocation list
- `host.crt` - Host certificate
- `host.key` - Host private key

### Validation Checks
1. Size limit: Maximum 10 MiB (10,485,760 bytes)
2. Format: Valid gzip tar archive
3. Content: Not empty
4. Required files: All 5 files must be present
5. YAML syntax: config.yml must parse without errors

### Error Handling
- `ErrBundleTooLarge` - Bundle exceeds 10 MiB
- `ErrInvalidFormat` - Not a valid gzip tar archive
- `ErrEmptyBundle` - Archive contains no files
- `ErrMissingRequiredFile` - Missing one or more required files
- `ErrInvalidYAML` - config.yml has YAML syntax errors

All errors are wrapped with additional context for debugging.

## Version Management

### Strategy
- Last-write-wins for concurrent uploads
- Incremental versioning (1, 2, 3, ...)
- Atomic version increment using database transactions
- Version stored in `clusters.config_version`

### Conditional Requests
Supports HTTP caching via ETag:
- Client sends `If-None-Match: "vN"` header
- Server compares with current version
- Returns 304 Not Modified if client has latest version
- Reduces bandwidth for nodes polling for updates

## Testing Results

### Bundle Validator Tests
```
TestValidate_ValidBundle                 PASS
TestValidate_BundleTooLarge             PASS
TestValidate_InvalidGzip                PASS
TestValidate_EmptyBundle                PASS
TestValidate_MissingRequiredFile        PASS
TestValidate_InvalidYAML                PASS
TestValidate_ValidBundleWithExtraFiles  PASS
TestValidate_InvalidTarArchive          PASS
```

### Bundle Service Tests
```
TestBundleService_UploadAndGetVersion           PASS
TestBundleService_UploadInvalidBundle          PASS
TestBundleService_UploadMissingRequiredFile    PASS
TestBundleService_Download                      PASS
TestBundleService_DownloadSpecificVersion      PASS
TestBundleService_CheckVersion                 PASS
TestBundleService_MultipleUploads              PASS
```

### Build Status
- ✅ Server builds successfully
- ✅ All imports resolved
- ✅ No import cycles
- ✅ All tests passing

## Security Features

1. **Size Limits**: Enforced at multiple layers to prevent resource exhaustion
2. **Admin-Only Uploads**: Requires admin node authentication
3. **Rate Limiting**: 10 req/s limit on config endpoints
4. **Input Validation**: Comprehensive validation of bundle contents
5. **Generic Errors**: HTTP responses use generic messages to prevent info disclosure

## Standards Compliance

- ✅ All functions have documentation comments
- ✅ All structs and fields documented
- ✅ Error handling with wrapped context
- ✅ Input validation at all layers
- ✅ Admin-only upload operations
- ✅ Rate limiting applied
- ✅ Tests written for all components
- ✅ No code duplication

## Database Schema Used

### config_bundles Table
```sql
CREATE TABLE config_bundles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id TEXT NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    data BLOB NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE(cluster_id, version)
);
```

## Integration Points

### Upstream Dependencies
- Authentication middleware (node token validation)
- Cluster context (getClusterID from request)
- Database connection
- Logger (Zap)

### Downstream Consumers
- Task 00011: Lighthouse process manager will poll for bundle updates
- Client SDK: Will use these endpoints for config synchronization
- Node daemon: Will download bundles periodically

## Performance Considerations

1. **Caching**: 304 Not Modified responses minimize bandwidth
2. **Streaming**: Large bundles use c.Data() for efficient transfer
3. **Transactions**: Atomic version updates prevent race conditions
4. **Size Limits**: Prevents memory exhaustion from large uploads

## Next Steps

The following tasks depend on this implementation:
- **Task 00009**: Topology management (lighthouse/relay assignment)
- **Task 00011**: Lighthouse process management (will use bundle updates)

## Notes

Config bundles are the primary distribution mechanism for Nebula configurations. Nodes poll the `/api/v1/config/version` endpoint every 5 seconds to check for updates. When a new version is detected, they download the full bundle and restart their Nebula instances.

The validation ensures that bundles contain all necessary files for a working Nebula configuration, preventing runtime errors from incomplete configurations.
