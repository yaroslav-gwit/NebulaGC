# Task 00006: High Availability Architecture

**Status**: In Progress
**Started**: 2025-01-21
**Phase**: 1 (HA Control Plane Core)
**Dependencies**: Task 00003 (Database), Task 00005 (REST API Foundation)

## Objective

Implement the high availability (HA) architecture for the control plane with:
- Replica registration and management
- Heartbeat mechanism for health monitoring
- Master election logic (oldest healthy replica)
- Automatic failover detection
- Replica pruning (remove stale replicas)

## Scope

### Files to Create
- `server/internal/service/replica.go` - Replica service with registration, heartbeat, master election
- `server/internal/ha/manager.go` - HA manager that runs heartbeat goroutine
- `server/internal/ha/types.go` - HA-specific types and constants

### Key Functions
- `RegisterReplica()` - Register this instance in replicas table
- `SendHeartbeat()` - Update last_heartbeat timestamp
- `GetMasterURL()` - Determine current master and return URL
- `PruneStaleReplicas()` - Remove replicas with old heartbeats
- `StartHeartbeat()` - Background goroutine for periodic heartbeats

## Implementation Details

### Master Election Algorithm
1. Query all replicas with `last_heartbeat > (now - threshold)`
2. Sort by `created_at ASC` (oldest first)
3. First replica in list is the master
4. This provides deterministic, consistent master selection

### Heartbeat Mechanism
- Default interval: 10 seconds
- Threshold for staleness: 30 seconds (3x interval)
- Background goroutine runs for lifetime of server
- Graceful shutdown on SIGTERM/SIGINT

### Replica Registration
On server startup:
1. Check if instance_id already exists in replicas table
2. If exists: update URL and heartbeat (restart scenario)
3. If not exists: insert new replica record
4. Start heartbeat goroutine

### Pruning Strategy
- Run periodically (every 5 minutes)
- Remove replicas with `last_heartbeat < (now - threshold * 2)`
- Prevents table from growing indefinitely
- Double threshold to avoid race conditions

## Standards Compliance

- ✅ All functions have documentation comments
- ✅ All structs and fields documented
- ✅ Errors wrapped with context
- ✅ Zap structured logging
- ✅ Graceful shutdown handling
- ✅ Thread-safe operations

## Testing Strategy

- Unit tests for master election logic
- Test heartbeat mechanism with time mocking
- Test pruning with stale replicas
- Integration test with multiple instances
- Verify failover behavior

## Progress

- [x] Create HA types and constants
- [x] Implement replica service
- [x] Implement HA manager with heartbeat
- [x] Add replica registration to server startup
- [x] Add graceful shutdown handling
- [x] Integrate with replica middleware
- [ ] Write unit tests
- [ ] Update task documentation

## Notes

This task completes the HA foundation that was partially implemented in Task 00005 (replica write guard middleware). The middleware checks if this instance is master; this task implements the actual registration and heartbeat that makes that check work.

After this task, the control plane will support N-way replication with automatic master election and failover.
