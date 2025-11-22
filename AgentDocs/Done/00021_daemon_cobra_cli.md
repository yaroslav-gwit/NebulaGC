# Task 00021: Daemon Cobra CLI

**Status**: In Progress
**Priority**: Medium
**Complexity**: Medium
**Dependencies**: Task 00020 (Nebula Process Supervision)

## Objective

Create a command-line interface for the NebulaGC daemon with status monitoring and management capabilities.

## Deliverables

### Core Components
- `cmd/nebulagc/main.go` - CLI entry point with Cobra root command
- `cmd/nebulagc/cmd/daemon.go` - Daemon mode command (run in foreground)
- `cmd/nebulagc/cmd/version.go` - Version information

### Commands

#### Primary Command
```bash
nebulagc daemon --config /etc/nebulagc/config.json
```
- Starts daemon in foreground
- Manages all configured clusters
- Logs to stdout (structured JSON with Zap)
- Handles SIGTERM/SIGINT for graceful shutdown

#### Version Command
```bash
nebulagc version
```
- Shows version, commit, build date
- Displays Go version used for build

## Architecture

### Command Structure
```
nebulagc
├── daemon [--config PATH]     # Start daemon
└── version                    # Show version info
```

### Build Info Integration
Version information from:
- `Version`: Set via `-ldflags` at build time
- `Commit`: Git commit hash
- `BuildDate`: Build timestamp
- `GoVersion`: Runtime Go version

### Logging
- Use `go.uber.org/zap` for structured logging
- Production mode: JSON output
- Development mode: Console output (optional)
- Log levels: Debug, Info, Warn, Error

## Implementation Plan

1. Initialize Cobra CLI structure in `cmd/nebulagc/`
2. Create root command with global flags
3. Implement `daemon` command
4. Implement `version` command
5. Integrate with existing daemon code
6. Add build-time version injection
7. Update Makefile for build

## Testing Strategy

1. **CLI Tests**
   - Verify command structure
   - Test flag parsing
   - Validate help output

2. **Integration Tests**
   - Start daemon with valid config
   - Stop daemon with SIGTERM
   - Invalid config handling

3. **Manual Testing**
   - Run daemon with test config
   - Verify logs format
   - Check version output

## Success Criteria

- ✅ `nebulagc daemon` starts successfully with valid config
- ✅ Daemon logs to stdout in JSON format
- ✅ Graceful shutdown on SIGTERM/SIGINT
- ✅ `nebulagc version` shows build info
- ✅ Help text is clear and accurate
- ✅ Invalid config shows helpful error messages
- ✅ Binary builds with version info embedded

## Implementation Details

### Files Created

**cmd/nebulagc/main.go** (14 lines)
- Entry point for CLI
- Calls `cmd.Execute()` and handles errors

**cmd/nebulagc/cmd/root.go** (48 lines)
- Root Cobra command with description
- Version variables for build-time injection
- Global command structure

**cmd/nebulagc/cmd/version.go** (22 lines)
- Version command showing:
  - Version (from git describe or "dev")
  - Commit hash
  - Build timestamp
  - Go runtime version

**cmd/nebulagc/cmd/daemon.go** (101 lines)
- Daemon command with flags:
  - `--config/-c`: Config file path (default: `/etc/nebulagc/config.json`)
  - `--dev`: Development mode (console logging vs JSON)
- Creates Manager with logger
- Runs manager (blocks until shutdown)
- Manager handles SIGTERM/SIGINT internally

### Makefile Updates

Added version injection via `-ldflags`:
```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS := -ldflags "\
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.Version=$(VERSION)' \
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.Commit=$(COMMIT)' \
	-X 'github.com/yaroslav/nebulagc/cmd/nebulagc/cmd.BuildDate=$(BUILD_DATE)'"
```

### Logger Configuration

Production mode (default):
- JSON structured logging
- ISO8601 timestamps
- Outputs to stdout

Development mode (`--dev`):
- Console formatting with colors
- Human-readable output
- Better for local development

### Integration with Daemon

The CLI integrates seamlessly with the existing daemon code:
1. `daemon.NewManager()` loads config and initializes
2. `manager.Run()` starts cluster managers and handles signals internally
3. Graceful shutdown on SIGTERM/SIGINT via Manager's signal handler
4. All logs flow through zap logger provided to Manager

## Test Results

### Manual Testing

**Version Command**:
```bash
$ ./bin/nebulagc version
NebulaGC d39c329-dirty
Commit: d39c329
Built: 2025-11-21_22:06:35
Go: go1.22.0
```

**Help Output**:
```bash
$ ./bin/nebulagc --help
# Shows root help with available commands

$ ./bin/nebulagc daemon --help
# Shows daemon command help with flags
```

**Error Handling**:
```bash
$ ./bin/nebulagc daemon --config /tmp/nonexistent.json
# Logs JSON error message
# Exits with error code 1
# Shows clear error about missing file
```

**Build with Version Info**:
```bash
$ make build
Version: d39c329-dirty
Commit: d39c329
Build Date: 2025-11-21_22:06:35
# Binary includes embedded version information
```

### Success Criteria Verification

- ✅ CLI compiles without errors
- ✅ Version command shows git commit, date, Go version
- ✅ Daemon command accepts --config flag
- ✅ Development mode flag (--dev) works
- ✅ JSON logging in production mode
- ✅ Console logging in dev mode
- ✅ Helpful error messages for invalid config
- ✅ Manager integration works correctly
- ✅ Makefile builds with version injection
