# NebulaGC Implementation Quick Start Guide

## For New AI Agents or Developers

This guide helps you quickly understand where to start and what to work on next.

---

## Current Project State

- **Phase**: Planning/Specification Complete
- **Implementation Status**: Not started
- **Next Task**: Task 00001 (Project Structure Setup)
- **Total Tasks**: 32 tasks across 3 phases

---

## Quick Start Checklist

### 1. Mandatory Reading (15 minutes)
Read these files in order:
1. ✅ [claude.md](../../claude.md) - Project overview (you're here)
2. ✅ [AgentDocs/constitution.md](../constitution.md) - Coding standards
3. ✅ [AgentDocs/ToDo/nebula_control_plane_spec.md](nebula_control_plane_spec.md) - Technical spec

### 2. Understand Task System (5 minutes)
- Tasks in `ToDo/` are unnumbered specifications
- Move to `InProgress/` and assign next number: `XXXXX_task_name.md`
- Complete work, then move to `Done/` keeping same number
- Numbers enable easy rollback and change tracking

### 3. Check Current Status (2 minutes)
```bash
# See what's been done
ls AgentDocs/Done/

# See what's in progress
ls AgentDocs/InProgress/

# Find highest task number
ls AgentDocs/Done/ | sort | tail -1
```

### 4. Pick Your Task (based on current state)
- **If no tasks in `Done/` or `InProgress/`**: Start with Task 00001
- **If tasks in progress**: Review dependencies and pick next available task
- **If uncertain**: Read [implementation_roadmap.md](implementation_roadmap.md)

---

## Task Selection Guide

### I want to work on backend/database
→ Start with Tasks 00001-00003 (Foundation)

### I want to work on REST API
→ Wait for Tasks 00001-00004 to complete, then pick Tasks 00005-00009

### I want to work on CLI
→ Wait for API completion (Task 00009), then pick Task 00010

### I want to work on client SDK
→ Wait for Phase 1 completion (Task 00011), then pick Tasks 00012-00016

### I want to work on daemon
→ Wait for SDK completion (Task 00016), then pick Tasks 00017-00022

### I want to work on DevOps/Deployment
→ Wait for Phase 2 completion (Task 00022), then pick Tasks 00023-00032

---

## Task File Template

When moving a task to `InProgress/`, create a file like this:

```markdown
# Task XXXXX: <Task Name>

## Status
- Started: YYYY-MM-DD
- Completed: In Progress

## Objective
[Brief description from phase breakdown]

## Changes Made
- [ ] File 1 created
- [ ] Function X implemented
- [ ] Tests written
- [ ] Documentation added

## Dependencies
- Task XXXXX must be completed first

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Manual testing completed

## Rollback Plan
- How to undo these changes if needed
```

---

## Essential Commands

### Database
```bash
# Generate SQLc code
cd server && sqlc generate

# Run migrations
goose -dir server/migrations sqlite3 ./nebula.db up

# Rollback migration
goose -dir server/migrations sqlite3 ./nebula.db down
```

### Build
```bash
# Build server
go build -o bin/nebulagc-server ./server/cmd/nebulagc-server

# Build daemon
go build -o bin/nebulagc ./cmd/nebulagc

# Build everything
go build ./...
```

### Testing
```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...
```

### Development
```bash
# Format code
gofmt -w ./...

# Run linter (if installed)
golangci-lint run ./...

# Tidy dependencies
go mod tidy
```

---

## Phase-Specific Quick References

### Phase 1: Control Plane Core
**Goal**: Working REST API with authentication, HA, and lighthouse management

**Key Files to Create**:
- `models/*.go` - Shared data structures
- `server/migrations/*.sql` - Database schema
- `server/queries/*.sql` - SQLc queries
- `server/internal/api/*.go` - REST API
- `server/internal/lighthouse/*.go` - Lighthouse management
- `server/cmd/nebulagc-server/*.go` - CLI

**Key Dependencies**:
- Gin (REST framework)
- SQLc (code generation)
- Goose (migrations)
- Zap (logging)
- Cobra (CLI)

### Phase 2: SDK and Daemon
**Goal**: Client SDK and multi-cluster daemon with auto-updates

**Key Files to Create**:
- `sdk/*.go` - Client library
- `cmd/nebulagc/daemon/*.go` - Daemon logic
- `cmd/nebulagc/cmd/*.go` - CLI commands

**Key Dependencies**:
- Phase 1 complete (REST API available)
- Bubble Tea (TUI)

### Phase 3: Production Hardening
**Goal**: Deployment-ready with monitoring, docs, and tooling

**Key Files to Create**:
- `docs/*.md` - Documentation
- `Makefile` - Build automation
- `.github/workflows/*.yml` - CI/CD
- `tests/e2e/*.go` - End-to-end tests

**Key Dependencies**:
- Phase 2 complete (full functionality)
- Prometheus (metrics)

---

## Common Pitfalls to Avoid

### ❌ Don't Do This
- Start coding without reading the constitution
- Skip task numbering system
- Create code without documentation
- Duplicate functionality that already exists
- Modify files without reading them first
- Log sensitive data (tokens, keys)
- Skip tests

### ✅ Do This Instead
- Read all mandatory documentation first
- Follow task numbering strictly
- Document every function, struct, and field
- Extract common code to shared packages
- Read files before editing
- Use structured logging (never log secrets)
- Write tests alongside code

---

## Getting Help

### Where to Find Information
- **Architecture questions**: [nebula_control_plane_spec.md](nebula_control_plane_spec.md)
- **Code standards**: [constitution.md](../constitution.md)
- **Task breakdown**: [phase1_task_breakdown.md](phase1_task_breakdown.md) (and phase2, phase3)
- **Overall roadmap**: [implementation_roadmap.md](implementation_roadmap.md)

### Common Questions

**Q: What task should I start with?**
A: Check `Done/` folder for highest number, pick next sequential task from phase breakdown.

**Q: Can I work on multiple tasks in parallel?**
A: Yes, if they don't have dependencies. See roadmap "Parallelization Opportunities".

**Q: What if I find an issue in the spec?**
A: Document it in the task file, discuss with team, update spec if needed.

**Q: How much test coverage is required?**
A: >80% overall, >95% for security-critical code.

**Q: Can I use different libraries than specified?**
A: Only with explicit approval and documented rationale.

**Q: What Go version should I use?**
A: Go 1.22+ (specified in spec)

---

## Success Criteria for Each Task

Before marking a task as "Done":
- ✅ All code compiles without errors
- ✅ All tests pass (including race detector)
- ✅ Every function has documentation
- ✅ Every struct and field documented
- ✅ Code follows constitution standards
- ✅ Task file updated with completion date
- ✅ Git commit created with task reference
- ✅ No sensitive data in logs or comments

---

## Phase Completion Criteria

### Phase 1 Complete When:
- Server runs with `--master` or `--replicate`
- All REST endpoints work
- Authentication enforced
- Lighthouses spawn and restart
- CLI manages tenants/clusters/nodes
- Tests >80% coverage

### Phase 2 Complete When:
- SDK handles all operations
- Daemon manages multiple clusters
- Polling detects config updates
- Process supervision works
- Failover handles failures
- E2E test passes

### Phase 3 Complete When:
- Rate limiting active
- Metrics exposed
- Deployment guides tested
- Security audit clean
- E2E tests comprehensive
- Documentation complete
- Ready for v1.0.0

---

## Next Steps

1. **Read mandatory documentation** (if you haven't already)
2. **Check current state**: `ls AgentDocs/Done/ AgentDocs/InProgress/`
3. **Pick next task**: Review phase breakdown for next available task
4. **Move task to InProgress**: Rename with next sequential number
5. **Start coding**: Follow constitution standards
6. **Test thoroughly**: Write and run tests
7. **Document everything**: Functions, structs, fields
8. **Move to Done**: Update task file with completion date
9. **Commit**: Reference task number in commit message
10. **Repeat**: Pick next task

---

Good luck! Remember: **Quality over speed**. Following the standards ensures long-term maintainability.

---

Last Updated: 2025-01-21
