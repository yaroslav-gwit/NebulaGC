# Contributing to NebulaGC

Thank you for your interest in contributing to NebulaGC! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Task Management](#task-management)
- [Questions and Support](#questions-and-support)

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inspiring community for all. We expect all participants to:

- Use welcoming and inclusive language
- Be respectful of differing viewpoints and experiences
- Gracefully accept constructive criticism
- Focus on what is best for the community
- Show empathy towards other community members

### Unacceptable Behavior

- Harassment, trolling, or discriminatory comments
- Publishing others' private information
- Other conduct which could reasonably be considered inappropriate

## Getting Started

### Prerequisites

- Go 1.22 or later
- Git
- SQLite 3.x
- golangci-lint (for linting)

### Development Setup

```bash
# Clone repository
git clone https://github.com/yaroslav-gwit/nebulagc.git
cd nebulagc

# Build server
make build-server

# Run tests
make test

# Run linters
make lint
```

### Project Structure

```
NebulaGC/
├── server/               # Control plane server
│   ├── cmd/              # Server CLI
│   ├── internal/         # Server implementation
│   ├── migrations/       # Database migrations
│   └── queries/          # SQLc query files
├── sdk/                  # Go client SDK
├── cmd/nebulagc/         # Client daemon (planned)
├── models/               # Shared data models
├── pkg/                  # Reusable utilities
├── tests/                # E2E and benchmark tests
├── AgentDocs/            # Development documentation
│   ├── Planning/         # Task breakdowns
│   ├── InProgress/       # Active tasks
│   └── Done/             # Completed tasks
└── docs/                 # User-facing documentation
```

## Development Workflow

### 1. Find or Create an Issue

- Check [GitHub Issues](https://github.com/yaroslav-gwit/nebulagc/issues) for existing issues
- Create a new issue if your contribution doesn't have one
- Discuss significant changes before implementing

### 2. Fork and Branch

```bash
# Fork the repository on GitHub
git clone https://github.com/YOUR_USERNAME/nebulagc.git
cd nebulagc

# Create a feature branch
git checkout -b feature/your-feature-name
```

### 3. Make Changes

- Follow the coding standards (see below)
- Write tests for new functionality
- Update documentation as needed
- Run tests and linters before committing

```bash
# Run tests
make test

# Run linters
make lint

# Format code
make format
```

### 4. Commit Your Changes

Use clear, descriptive commit messages:

```bash
git add .
git commit -m "Add feature: brief description

Detailed explanation of what changed and why.
Fixes #123"
```

### 5. Push and Create Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Coding Standards

NebulaGC follows strict coding standards to ensure code quality. Full details are in [AgentDocs/constitution.md](AgentDocs/constitution.md).

### Key Requirements

**Documentation:**
- All exported functions, types, and constants must have doc comments
- Doc comments should be complete sentences starting with the item name
- Include examples for complex functionality

**Error Handling:**
- Always check errors explicitly
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Return errors to callers, don't log and continue
- Use sentinel errors for expected error conditions

**Testing:**
- All new code must have tests
- Aim for high coverage on critical paths
- Use table-driven tests for multiple scenarios
- Test both success and error cases

**Code Style:**
- Use `gofmt` for formatting (automatically applied by `make format`)
- Follow standard Go naming conventions
- Keep functions focused and small (< 50 lines when possible)
- Avoid global variables

**Security:**
- Never log sensitive data (tokens, passwords, keys)
- Use constant-time comparison for secrets
- Validate all input at API boundaries
- Use parameterized queries (SQLc enforces this)

### Example: Well-Documented Function

```go
// CreateNode creates a new node in the specified cluster.
//
// The node name must be unique within the cluster. If is_admin is true,
// the node will have administrative privileges. Returns the created node
// with its generated ID and token hash.
//
// Returns ErrDuplicateNode if a node with the same name already exists.
// Returns ErrClusterNotFound if the cluster does not exist.
func (s *Service) CreateNode(ctx context.Context, clusterID, name string, isAdmin bool) (*models.Node, error) {
    if err := validateNodeName(name); err != nil {
        return nil, fmt.Errorf("invalid node name: %w", err)
    }

    node, err := s.db.InsertNode(ctx, clusterID, name, isAdmin)
    if err != nil {
        return nil, fmt.Errorf("failed to insert node: %w", err)
    }

    return node, nil
}
```

## Testing Requirements

### Unit Tests

- Test all public functions
- Test error conditions
- Use table-driven tests for multiple scenarios
- Mock external dependencies

```go
func TestCreateNode(t *testing.T) {
    tests := []struct {
        name      string
        clusterID string
        nodeName  string
        isAdmin   bool
        wantErr   error
    }{
        {"valid node", "cluster-1", "node-1", false, nil},
        {"admin node", "cluster-1", "admin", true, nil},
        {"duplicate", "cluster-1", "node-1", false, ErrDuplicateNode},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

- Test component interactions
- Use test databases
- Clean up after tests

### E2E Tests

Located in `tests/e2e/`, these test complete workflows:

```bash
# Run E2E tests
make test-e2e

# Run with verbose output
make test-e2e-verbose
```

### Running Tests

```bash
# All tests
make test

# Unit tests only
go test ./server/... ./pkg/...

# E2E tests
make test-e2e

# With coverage
make test-coverage
```

## Pull Request Process

### Before Submitting

1. **Run tests**: `make test`
2. **Run linters**: `make lint`
3. **Format code**: `make format`
4. **Update documentation**: If you changed APIs or behavior
5. **Update CHANGELOG.md**: Add entry under "Unreleased"

### PR Description Template

```markdown
## Description
Brief description of changes

## Related Issue
Fixes #123

## Changes Made
- Added feature X
- Fixed bug Y
- Updated documentation Z

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed

## Checklist
- [ ] Code follows project standards
- [ ] All tests pass
- [ ] Linters pass
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
```

### Review Process

1. Automated checks run (tests, linters)
2. Code review by maintainers
3. Address feedback
4. Approval and merge

### Merge Criteria

- All tests pass
- No linter errors
- Approved by at least one maintainer
- Documentation updated
- CHANGELOG.md updated

## Task Management

NebulaGC uses a task-based workflow documented in `AgentDocs/`.

### Task Lifecycle

1. **Planning**: Tasks defined in `AgentDocs/Planning/`
2. **Todo**: Ready tasks in `AgentDocs/ToDo/`
3. **In Progress**: Active tasks in `AgentDocs/InProgress/00XXX_task_name.md`
4. **Done**: Completed tasks in `AgentDocs/Done/00XXX_task_name.md`

### Task Document Format

Each task has a markdown file with:
- Objective and deliverables
- Success criteria
- Implementation notes
- Progress tracking

### Starting a New Task

1. Choose task from `AgentDocs/ToDo/`
2. Move to `AgentDocs/InProgress/` with next sequential number
3. Update task status to "In Progress"
4. Implement according to specification
5. Move to `AgentDocs/Done/` when complete

## Questions and Support

### Getting Help

- **Issues**: Use GitHub Issues for bug reports and feature requests
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: Check `AgentDocs/` and `docs/` directories

### Reporting Bugs

Include:
- NebulaGC version
- Go version
- Operating system
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs

### Suggesting Features

Include:
- Use case description
- Proposed solution
- Alternative approaches considered
- Impact on existing functionality

## License

By contributing to NebulaGC, you agree that your contributions will be licensed under the MIT License.

## Recognition

Contributors are recognized in:
- GitHub contributor list
- Release notes for significant contributions
- CHANGELOG.md for feature additions

## Additional Resources

- [Technical Specification](AgentDocs/Planning/nebula_control_plane_spec.md)
- [Implementation Roadmap](AgentDocs/Planning/implementation_roadmap.md)
- [Coding Standards](AgentDocs/constitution.md)
- [Progress Tracking](PROGRESS.md)

---

Thank you for contributing to NebulaGC! 🚀
