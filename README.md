# NebulaGC (Nebula Ground Control)

Lightweight control plane for Nebula that keeps configs versioned, signed, and distributed to nodes. REST is intentionally **cluster-scoped** (anon-friendly): node enrollment and bundle delivery only. Tenant/cluster lifecycle plus global listings live behind the server’s Cobra CLI or a privileged Unix socket.

## Features
- Cluster-scoped REST API for node enrollment and config bundle upload/download.
- Super-admin CLI (and optional Unix socket) for tenant/cluster/node lifecycle and PKI rotation.
- Daemon (`nebulagc`) that polls, unpacks bundles, and supervises Nebula processes per cluster.
- SQLite + Goose migrations + SQLc; Go stack with Gin, Cobra, Bubble Tea tables (with `--output json`).
- Versioned config bundles (`tar.gz`) with Nebula config + certs; version headers for caching.

## Repo Layout
- `server/cmd/nebulagc-server`: Cobra entrypoint (`serve`, `tenant|cluster|node create`, `list`, etc.).
- `server/internal/{api,auth,db,service}`: HTTP handlers, auth, SQLc code, business logic.
- `server/migrations`: Goose SQL migrations.
- `sdk`: Go client for cluster-scoped REST (hides headers/JSON).
- `cmd/nebulagc`: Daemon/CLI for cluster admins; supervises Nebula processes.

## Prerequisites
- Go 1.22+
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `go install github.com/pressly/goose/v3/cmd/goose@latest`

## Building
```bash
# Generate SQLc code
cd server && sqlc generate

# Run migrations (dev)
goose -dir server/migrations sqlite3 ./nebula.db up

# Build server and daemon
go build -o bin/nebulagc-server ./server/cmd/nebulagc-server
go build -o bin/nebulagc ./cmd/nebulagc
```

## Running
```bash
# Start server (REST + optional admin socket)
./bin/nebulagc-server serve --http :8080 --db ./nebula.db \
  --admin-socket /var/run/nebulagc_admin.sock   # optional

# Start daemon (cluster-admin scope)
./bin/nebulagc daemon --config ./dev_config.json
```

## Super-Admin Workflow (CLI / Unix Socket)
1. Bootstrap (once): `nebulagc-server tenant create --name Acme --bootstrap-token $NEBULA_BOOTSTRAP_TOKEN`
2. Add cluster: `nebulagc-server cluster create --tenant-id <tid> --name prod`
3. Create admin node: `nebulagc-server node create --tenant-id <tid> --cluster-id <cid> --name admin-1 --role admin`
4. Hand credentials + bundle to tenant admin.
5. Listing/inspection: `nebulagc-server list tenants|clusters|nodes --output json` (Bubble Tea tables by default).

## Cluster-Scoped REST (for admins/nodes)
- `POST /v1/tenants/{tenant}/clusters/{cluster}/nodes` (admin of that cluster) → returns node token once.
- `GET /v1/config/version` → latest version.
- `GET /v1/config/bundle?current_version=X` → 304 or tar.gz with `X-Nebula-Config-Version`.
- `POST /v1/config/bundle` (admin of that cluster) → uploads new version (cert rotation, etc.).
- `GET /v1/healthz` → liveness.

## Config Bundle Contract
- `config.yml`, `ca.crt`, `crl.pem`, `host.crt`, `host.key`; optional `lighthouses.json`.
- Max size 10 MiB; server stores verbatim and bumps `(tenant,cluster)` version.

## CLI Output Modes
- Bubble Tea tables for human-friendly views.
- `--output json` for scripting (applies to `nebulagc-server` and `nebulagc`).

## Notes
- REST cannot list tenants/clusters; only cluster-local operations are exposed to keep it anon-friendly.
- Admin Unix socket is disabled by default; enable with `--admin-socket` or env var (e.g. `NEBULAGC_ADMIN_SOCKET_PATH`).
