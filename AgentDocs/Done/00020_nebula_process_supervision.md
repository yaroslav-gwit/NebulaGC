# Task 00020: Nebula Process Supervision

**Status**: In Progress
**Priority**: High
**Complexity**: Medium-High
**Dependencies**: Task 00019 (Config Poller and Bundle Management)

## Objective

Implement process supervision for Nebula instances with automatic restart logic, crash detection, log capture, and integration with config update mechanisms.

## Deliverables

### Core Components
- `cmd/nebulagc/daemon/supervisor.go` - Process lifecycle management
- `cmd/nebulagc/daemon/process.go` - Process wrapper and monitoring
- Integration with `ClusterManager` for config update handling

### Features
- **Process Startup**: Start Nebula with correct config path
- **Health Monitoring**: Track process state and detect crashes
- **Log Capture**: Capture stdout/stderr with structured logging
- **Crash Recovery**: Automatic restart with exponential backoff
- **Config Updates**: Restart on config bundle updates
- **Graceful Shutdown**: SIGTERM handling for clean exits

## Process Command

```bash
nebula -config /etc/nebula/<cluster-name>/config.yml
```

## Architecture

### Supervisor
- Manages Nebula process lifecycle
- Implements restart logic with backoff
- Coordinates with ClusterManager for config updates
- Handles graceful shutdown

### Process Wrapper
- Wraps `exec.Cmd` for Nebula process
- Captures stdout/stderr to structured logs
- Monitors process health
- Provides start/stop/restart methods

### Restart Backoff Strategy
- Initial delay: 1 second
- Max delay: 60 seconds
- Exponential backoff: delay * 2
- Reset on successful run (> 5 minutes)

## Testing Strategy

1. **Process Lifecycle Tests**
   - Start Nebula process successfully
   - Stop process gracefully (SIGTERM)
   - Force kill if SIGTERM fails

2. **Crash Detection Tests**
   - Detect process crash (exit code != 0)
   - Trigger automatic restart
   - Apply backoff delays correctly

3. **Config Update Tests**
   - Restart on config bundle update
   - Maintain process state across restarts

4. **Log Capture Tests**
   - Capture stdout/stderr
   - Forward to structured logger
   - Handle process output correctly

5. **Graceful Shutdown Tests**
   - Stop supervisor cleanly
   - Ensure Nebula process stops
   - No orphaned processes

## Implementation Plan

1. Create `process.go` with Process struct
2. Create `supervisor.go` with Supervisor struct
3. Integrate with ClusterManager
4. Add comprehensive tests
5. Validate with manual testing

## Success Criteria

- ✅ Nebula process starts correctly
- ✅ Process crash triggers restart with backoff
- ✅ Config update triggers restart
- ✅ Logs captured and forwarded
- ✅ Graceful stop works (SIGTERM)
- ✅ No restart loops (backoff working)
- ✅ Tests passing with >70% coverage

## Implementation Details

### process.go (196 lines)
Process wrapper for Nebula with lifecycle management:
- **NewProcess()**: Creates process wrapper with config path and logger
- **Start()**: Starts Nebula with `exec.CommandContext`, captures stdout/stderr pipes
- **Wait()**: Blocks until process exits, returns exit code or signal
- **Stop()**: Sends SIGTERM for graceful shutdown, waits up to 10 seconds, then force kills
- **IsRunning()**: Thread-safe check if process is running
- **PID()**: Returns current process ID
- **captureOutput()**: Background goroutine that reads stdout/stderr and logs with structured logging

### supervisor.go (219 lines)
Supervisor manages process lifecycle with restart logic:
- **NewSupervisor()**: Creates supervisor with configurable backoff settings
- **Run()**: Main supervision loop that starts process, waits for exit, applies backoff, restarts
- **Stop()**: Cancels context to stop supervisor, stops managed process gracefully
- **Restart()**: Sends signal to stop current process and start new one immediately
- **startProcess()**: Creates new Process instance and starts it
- **applyBackoff()**: Applies exponential backoff delay before restart (1s → 2s → 4s → ... → 60s max)
- **IsRunning()**: Returns whether supervised process is running
- **PID()**: Returns process ID of supervised process

### Integration with ClusterManager
Updated `cluster.go` to:
1. Initialize Supervisor with config path from ClusterManager
2. Wrap bundle update callback to trigger restart after applying new config
3. Start supervisor in goroutine alongside poller
4. Stop supervisor gracefully on shutdown

### Restart Backoff Strategy
- **Initial delay**: 1 second
- **Max delay**: 60 seconds
- **Growth**: Exponential (delay × 2)
- **Reset**: After process runs successfully for 5 minutes

### Log Capture
All Nebula stdout/stderr captured and logged with:
- `source`: "stdout" or "stderr"
- `line`: The actual log line
- `pid`: Process ID
- Cluster context (from ClusterManager logger)

## Test Results

### Process Tests (6 tests)
- `TestProcess_StartStop`: Start/stop lifecycle
- `TestProcess_Wait`: Wait for process exit
- `TestProcess_StartTwice`: Idempotent start
- `TestProcess_OutputCapture`: Stdout/stderr capture
- `TestProcess_StopNotRunning`: Stop on non-running process
- `TestProcess_MissingNebulaBinary`: Error handling for missing binary

### Supervisor Tests (7 tests)
- `TestSupervisor_StartStop`: Basic lifecycle
- `TestSupervisor_AutoRestart`: Automatic restart on crash
- `TestSupervisor_BackoffIncreases`: Exponential backoff verification
- `TestSupervisor_Restart`: Manual restart signal
- `TestSupervisor_BackoffReset`: Reset after successful run
- `TestSupervisor_GracefulShutdown`: SIGTERM handling
- `TestSupervisor_MultipleRestarts`: Multiple restart requests

### Coverage
- **Total daemon tests**: 56 tests
- **Coverage**: 77.9% of statements
- **All tests**: PASS
- **Test duration**: ~46 seconds
