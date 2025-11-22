# Task 00017: Daemon Configuration and Initialization

**Status**: Completed
**Started**: 2025-11-21
**Completed**: 2025-11-21
**Phase**: 2 (Node-Facing SDK and Daemon)
**Dependencies**: Task 00016 (SDK Replica Discovery)

## Objective

Create the daemon configuration file format, loading, validation, and initialization logic.

## Implementation Plan

### Files to Create

1. **cmd/nebulagc/daemon/config.go** - Configuration structures and loading
2. **cmd/nebulagc/daemon/config_test.go** - Configuration tests
3. **cmd/nebulagc/daemon/init.go** - Daemon initialization logic

### Configuration Format

```json
{
  "control_plane_urls": ["https://control1.example.com"],
  "clusters": [
    {
      "name": "prod-cluster",
      "tenant_id": "uuid",
      "cluster_id": "uuid",
      "node_id": "uuid",
      "node_token": "secret",
      "cluster_token": "secret",
      "config_dir": "/etc/nebula/prod-cluster"
    }
  ]
}
```

### Validation Rules

- `control_plane_urls`: Required, non-empty array of valid URLs
- `clusters`: Required, non-empty array
- `name`: Required, non-empty string
- `tenant_id`, `cluster_id`, `node_id`: Valid UUIDs
- `node_token`, `cluster_token`: Minimum 41 characters
- `config_dir`: Valid directory path

### Configuration Locations

1. `/etc/nebulagc/config.json` (production)
2. `./dev_config.json` (development, optional)
3. Custom path via flag or environment variable

## Changes Made

### cmd/nebulagc/daemon/config.go (219 lines)
- DaemonConfig struct with ControlPlaneURLs and Clusters array
- ClusterConfig struct with all required fields (name, IDs, tokens, config_dir)
- LoadConfig() - loads from dev_config.json (priority) or /etc/nebulagc/config.json
- LoadConfigFromPath() - loads from custom path
- DaemonConfig.Validate() - validates control plane URLs and all clusters
- ClusterConfig.Validate() - validates UUIDs, token lengths, config directory paths
- isValidUUID() - regex-based UUID validation (8-4-4-4-12 format)

### cmd/nebulagc/daemon/init.go (124 lines)
- Daemon struct with Config and SDK Clients map
- Initialize() - creates daemon from config, initializes all SDK clients
- createSDKClient() - configures SDK client with retry logic
- GetClient() - retrieves SDK client by cluster name
- GetClusterConfig() - retrieves cluster config by name
- ClusterNames() - returns list of configured clusters

### cmd/nebulagc/daemon/config_test.go (395 lines)
- TestDaemonConfig_Validate (7 test cases)
- TestClusterConfig_Validate (9 test cases)
- TestLoadConfig (4 test cases)
- TestIsValidUUID (10 test cases)

**Test Results**: All 30 tests passed, 52.0% coverage

## Testing

✅ Valid config loads successfully
✅ Valid config with cluster token
✅ Multiple clusters configuration
✅ Missing required fields return error
✅ Invalid UUIDs return error (tenant_id, cluster_id, node_id)
✅ Short tokens return error (node_token, cluster_token)
✅ Empty/invalid URLs return error
✅ Config directory validation (absolute path required)
✅ Development config can be loaded from custom path
✅ JSON parsing errors handled
✅ Config validation errors reported clearly
✅ UUID format validation (8-4-4-4-12, case-insensitive)

## Rollback Plan

- Remove cmd/nebulagc/daemon/config.go
- Remove cmd/nebulagc/daemon/config_test.go
- Remove cmd/nebulagc/daemon/init.go
- No database or migration changes in this task

## Notes

- Config file format uses JSON for simplicity and compatibility
- Development config (`dev_config.json`) overrides production config for testing
- UUID validation uses standard format: 8-4-4-4-12 hex digits
- Token length validation ensures HMAC-SHA256 tokens (41+ chars)
- Each cluster can have its own config directory for Nebula files
