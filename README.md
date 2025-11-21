# NebulaGC (Nebula Ground Control)

**A lightweight, multi-tenant control plane for managing Nebula overlay networks**

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Status](https://img.shields.io/badge/Status-In%20Development-yellow)](https://github.com/yaroslav/nebulagc)

---

## Overview

NebulaGC provides centralized configuration management, certificate distribution, and node enrollment for [Nebula](https://github.com/slackhq/nebula) mesh networks. It's designed to be lightweight, embeddable, and suitable for integration with router platforms like Hoster, OPNSense, pfSense, and VyOS.

### Key Features

- 🏢 **Multi-Tenant**: Isolated tenant and cluster management
- 🔐 **Secure Authentication**: HMAC-SHA256 token hashing with cluster and node-level secrets
- 🌐 **High Availability**: Master/replica architecture with automatic failover
- 📦 **Config Versioning**: Incremental config bundles with automatic updates
- 🗼 **Built-in Lighthouses**: Control plane can act as N-way redundant lighthouse
- 🔄 **Auto-Updates**: Client daemon polls every 5 seconds and restarts Nebula on changes
- 🛣️ **Route Management**: Nodes can advertise internal networks to the cluster
- 📊 **Production Ready**: Rate limiting, structured logging, Prometheus metrics

---

## Project Status

**Current Phase**: Planning Complete, Implementation Starting

- ✅ Complete technical specification
- ✅ Task breakdown (32 tasks across 3 phases)
- 🚧 Implementation in progress (Task 00001/00032)

See [AgentDocs/Planning/implementation_roadmap.md](AgentDocs/Planning/implementation_roadmap.md) for details.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    NebulaGC Control Plane                   │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Master     │  │  Replica 1   │  │  Replica 2   │      │
│  │  (Writes)    │  │   (Reads)    │  │   (Reads)    │      │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│         └─────────────────┴─────────────────┘               │
│                           │                                 │
│                   SQLite Database                           │
│              (Replicated via Litestream/LiteFS)             │
└─────────────────────────────────────────────────────────────┘
                           │
          ┌────────────────┼────────────────┐
          │                │                │
     ┌────▼─────┐    ┌────▼─────┐    ┌────▼─────┐
     │  Node 1  │    │  Node 2  │    │  Node 3  │
     │ (nebulagc│    │ (nebulagc│    │ (nebulagc│
     │  daemon) │    │  daemon) │    │  daemon) │
     └──────────┘    └──────────┘    └──────────┘
```

### Components

1. **Control Plane Server** (`nebulagc-server`)
   - REST API for node enrollment and config distribution
   - CLI for super-admin operations (tenant/cluster management)
   - Lighthouse process management
   - Master/replica HA support

2. **Client Daemon** (`nebulagc`)
   - Manages multiple Nebula instances per node
   - Polls control plane every 5 seconds for updates
   - Downloads and applies config bundles automatically
   - Supervises Nebula processes

3. **Go SDK** (`sdk/`)
   - Client library for programmatic access
   - HA-aware (automatic failover)
   - Used by daemon and external tools

---

## Quick Start

### Prerequisites

- Go 1.22 or later
- SQLite 3.x (included via modernc.org/sqlite)
- Optional: golangci-lint for linting

### Installation

```bash
# Clone repository
git clone https://github.com/yaroslav/nebulagc.git
cd nebulagc

# Install development tools
make install-tools

# Build binaries
make build
```

### Development Setup

```bash
# Start development server (master mode)
make dev-server

# In another terminal, start daemon (requires dev_config.json)
make dev-daemon
```

---

## Documentation

### For Developers

- **[claude.md](claude.md)**: Project overview and AI agent guide
- **[AgentDocs/constitution.md](AgentDocs/constitution.md)**: Coding standards and conventions
- **[AgentDocs/Planning/](AgentDocs/Planning/)**: Implementation roadmap and task breakdowns

### Technical Specification

- **[nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md)**: Complete REST API, database schema, and architecture details (1,386 lines)

---

## Development

### Common Commands

```bash
# Build everything
make build

# Run tests
make test

# Generate coverage report
make test-coverage

# Format code
make format

# Run linters
make lint

# Generate SQLc code
make generate

# Apply database migrations
make migrate

# Clean build artifacts
make clean
```

### Project Structure

```
NebulaGC/
├── server/               # Control plane server
│   ├── cmd/              # Server CLI entry point
│   ├── internal/         # Server implementation
│   ├── migrations/       # Database migrations
│   └── queries/          # SQLc query files
├── sdk/                  # Go client SDK
├── cmd/nebulagc/         # Client daemon
├── models/               # Shared data models
├── pkg/                  # Reusable utilities
├── AgentDocs/            # Development workflow
│   ├── Planning/         # Task breakdowns and roadmap
│   ├── ToDo/             # Tasks ready to start
│   ├── InProgress/       # Active tasks (numbered)
│   └── Done/             # Completed tasks (numbered)
└── Makefile              # Build automation
```

---

## Implementation Roadmap

The project is divided into **32 tasks** across 3 phases:

### Phase 1: HA Control Plane Core (Tasks 00001-00011) - ~2 weeks
- Project structure and models
- Database migrations and SQLc
- Authentication and REST API
- Master/replica HA architecture
- Server CLI and lighthouse management

### Phase 2: SDK and Daemon (Tasks 00012-00022) - ~1 week
- Go client SDK with HA support
- Multi-cluster daemon
- Config polling and auto-updates
- Nebula process supervision

### Phase 3: Production Hardening (Tasks 00023-00032) - ~1 week
- Rate limiting and security
- Structured logging and metrics
- Deployment guides
- End-to-end testing
- Documentation

See [implementation_roadmap.md](AgentDocs/Planning/implementation_roadmap.md) for details.

---

## Contributing

We follow strict coding standards to ensure code quality and maintainability.

### Before Contributing

1. Read [AgentDocs/constitution.md](AgentDocs/constitution.md) - Coding standards
2. Review [AgentDocs/Planning/quick_start_guide.md](AgentDocs/Planning/quick_start_guide.md)
3. Check task status in `AgentDocs/InProgress/` and `AgentDocs/Done/`

### Contribution Workflow

1. Pick a task from `AgentDocs/ToDo/`
2. Move to `AgentDocs/InProgress/` with next sequential number
3. Implement according to standards (all functions documented, tests written)
4. Ensure tests pass: `make test`
5. Move to `AgentDocs/Done/` when complete
6. Create PR with task reference

---

## Security

### Token Security
- All tokens minimum 41 characters (cryptographically random)
- HMAC-SHA256 hashing with server secret
- Constant-time comparison prevents timing attacks
- Tokens never logged except in debug mode

### API Security
- Cluster-scoped authentication (multi-layer)
- Rate limiting (per-IP and per-node)
- Generic error messages prevent information disclosure
- Super-admin operations isolated to CLI/Unix socket

---

## License

MIT License - See [LICENSE](LICENSE) for details

---

## Acknowledgments

- [Nebula](https://github.com/slackhq/nebula) by Slack - The overlay network we're managing
- [SQLc](https://sqlc.dev/) - Type-safe SQL code generation
- [Goose](https://github.com/pressly/goose) - Database migrations
- [Gin](https://gin-gonic.com/) - HTTP framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework

---

## Support

- **Issues**: [GitHub Issues](https://github.com/yaroslav/nebulagc/issues)
- **Documentation**: See `AgentDocs/` folder
- **Specification**: [nebula_control_plane_spec.md](AgentDocs/ToDo/nebula_control_plane_spec.md)

---

**Status**: In Development | **Version**: 0.1.0 | **Go**: 1.22+
