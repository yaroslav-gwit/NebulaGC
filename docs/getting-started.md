# Getting Started with NebulaGC

This guide will walk you through setting up NebulaGC and creating your first Nebula mesh network.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Initial Server Setup](#initial-server-setup)
- [Creating Your First Tenant and Cluster](#creating-your-first-tenant-and-cluster)
- [Creating Your First Node](#creating-your-first-node)
- [Uploading a Config Bundle](#uploading-a-config-bundle)
- [Verifying the Setup](#verifying-the-setup)
- [Next Steps](#next-steps)

## Prerequisites

### Software Requirements

- **Go 1.22 or later** - For building from source
- **SQLite 3.x** - Embedded database (included via CGO)
- **Linux, macOS, or Windows** - Cross-platform support

### Optional Requirements

- **Nebula binary** - For testing lighthouse functionality
- **Docker** - For containerized deployments
- **Kubernetes** - For production deployments

### Knowledge Requirements

- Basic understanding of [Nebula overlay networks](https://github.com/slackhq/nebula)
- Familiarity with REST APIs
- Basic Linux command-line skills

## Installation

### Option 1: Build from Source

```bash
# Clone the repository
git clone https://github.com/yaroslav-gwit/nebulagc.git
cd nebulagc

# Build the server
make build-server

# Verify the build
./bin/nebulagc-server --version
```

### Option 2: Using Pre-built Binaries

```bash
# Download latest release (when available)
curl -L https://github.com/yaroslav-gwit/nebulagc/releases/latest/download/nebulagc-server-linux-amd64 -o nebulagc-server
chmod +x nebulagc-server

# Verify
./nebulagc-server --version
```

### Option 3: Using Docker

```bash
# Pull the image (when available)
docker pull yaroslavgwit/nebulagc:latest

# Run container
docker run -d \
  -p 8080:8080 \
  -v /data/nebula:/data \
  -e NEBULAGC_HMAC_SECRET="your-secret-key-at-least-32-bytes-long" \
  yaroslavgwit/nebulagc:latest \
  --master --listen :8080 --db /data/nebula.db
```

## Initial Server Setup

### 1. Generate HMAC Secret

The HMAC secret is used for token hashing and must be at least 32 characters:

```bash
# Generate a secure random secret
export NEBULAGC_HMAC_SECRET=$(openssl rand -base64 32)
echo "Save this secret securely: $NEBULAGC_HMAC_SECRET"
```

**Important**: Save this secret! You'll need it for all server instances (master and replicas).

### 2. Start Master Instance

```bash
# Set required environment variables
export NEBULAGC_HMAC_SECRET="your-secret-key-at-least-32-bytes-long"
export NEBULAGC_PUBLIC_URL="https://nebulagc.example.com:8080"

# Start server in master mode
./bin/nebulagc-server \
  --master \
  --listen :8080 \
  --db ./nebula.db \
  --log-level info
```

The server will:
- Create the SQLite database (`nebula.db`)
- Apply all migrations automatically
- Start the REST API on port 8080
- Begin accepting requests

### 3. Verify Server is Running

```bash
# Check liveness
curl http://localhost:8080/health/liveness
# Expected: {"status":"ok"}

# Check master status
curl http://localhost:8080/health/master
# Expected: {"is_master":true}
```

## Creating Your First Tenant and Cluster

NebulaGC uses a multi-tenant architecture:
- **Tenant**: Top-level organization (e.g., your company)
- **Cluster**: A Nebula mesh network within a tenant
- **Node**: A member of the cluster

### 1. Create Tenant (Direct Database Access)

Currently, tenant creation requires direct database access. In a future version, this will be available via CLI.

```bash
# Using SQLite CLI
sqlite3 nebula.db "INSERT INTO tenants (id, name) VALUES ('tenant-001', 'My Organization')"
```

### 2. Create Cluster (Direct Database Access)

```bash
# Generate cluster token (41+ characters)
CLUSTER_TOKEN=$(openssl rand -base64 32)
echo "Cluster token: $CLUSTER_TOKEN"

# Hash the token (server will do this via API in future)
# For now, insert with placeholder hash
sqlite3 nebula.db "INSERT INTO clusters (id, tenant_id, name, token_hash) VALUES ('cluster-001', 'tenant-001', 'production', 'placeholder-hash')"

# Initialize cluster state
sqlite3 nebula.db "INSERT INTO cluster_state (cluster_id, config_version) VALUES ('cluster-001', 0)"
```

**Note**: Direct database access for tenant/cluster creation is temporary. These will be available via admin CLI commands in Task 00012.

## Creating Your First Node

Nodes are created via the REST API. The first node should be an admin node.

### 1. Create Admin Node

```bash
# Create admin node
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "X-NebulaGC-Cluster-Token: $CLUSTER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "admin-node",
    "is_admin": true
  }'
```

**Response:**
```json
{
  "id": "node-xxx",
  "cluster_id": "cluster-001",
  "name": "admin-node",
  "token": "very-long-random-token-41-characters-minimum",
  "is_admin": true,
  "mtu": 1300,
  "created_at": "2025-01-22T10:00:00Z"
}
```

**Important**: Save the token! It's only returned once during creation.

### 2. Create Regular Nodes

```bash
# Save admin token from previous response
ADMIN_TOKEN="very-long-random-token-41-characters-minimum"

# Create regular node
curl -X POST http://localhost:8080/api/v1/nodes \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "node-1",
    "is_admin": false
  }'
```

### 3. List All Nodes

```bash
curl http://localhost:8080/api/v1/nodes \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN"
```

## Uploading a Config Bundle

Config bundles contain Nebula configuration files (certificates, keys, config.yml).

### 1. Prepare Config Bundle

Create a directory with your Nebula configuration:

```bash
mkdir -p config-bundle
cd config-bundle

# Add your Nebula files
# - ca.crt (Certificate Authority)
# - host.crt (Node certificate)
# - host.key (Node private key)
# - config.yml (Nebula configuration)
```

Example `config.yml`:
```yaml
pki:
  ca: /etc/nebula/ca.crt
  cert: /etc/nebula/host.crt
  key: /etc/nebula/host.key

static_host_map:
  "192.168.100.1": ["1.2.3.4:4242"]

lighthouse:
  am_lighthouse: false
  interval: 60
  hosts:
    - "192.168.100.1"

listen:
  host: 0.0.0.0
  port: 4242

punchy:
  punch: true
  respond: true

tun:
  dev: nebula1
  drop_local_broadcast: false
  drop_multicast: false

firewall:
  outbound_action: drop
  inbound_action: drop
  
  conntrack:
    tcp_timeout: 12m
    udp_timeout: 3m
    default_timeout: 10m

  outbound:
    - port: any
      proto: any
      host: any

  inbound:
    - port: any
      proto: icmp
      host: any
```

### 2. Create tar.gz Bundle

```bash
cd config-bundle
tar -czf ../config-bundle.tar.gz *
cd ..
```

### 3. Upload Bundle

```bash
curl -X POST http://localhost:8080/api/v1/config/bundle \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/gzip" \
  --data-binary @config-bundle.tar.gz
```

**Response:**
```json
{
  "version": 1,
  "uploaded_at": "2025-01-22T10:05:00Z"
}
```

The config version is automatically incremented with each upload.

## Verifying the Setup

### 1. Check Config Version

```bash
curl http://localhost:8080/api/v1/config/version \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN"

# Expected: {"version":1}
```

### 2. Download Config Bundle

```bash
curl http://localhost:8080/api/v1/config/bundle \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN" \
  -o downloaded-bundle.tar.gz

# Verify bundle
tar -tzf downloaded-bundle.tar.gz
```

### 3. Check Node Routes (if configured)

```bash
curl http://localhost:8080/api/v1/routes \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN"
```

### 4. View Complete Topology

```bash
curl http://localhost:8080/api/v1/topology \
  -H "X-NebulaGC-Cluster-Token: $CLUSTER_TOKEN"
```

## Next Steps

### Deploy Additional Nodes

1. Create nodes via API (as shown above)
2. Distribute node tokens securely
3. Configure daemon on each node (when Task 00012 complete)

### Configure High Availability

1. Start replica instances
2. Configure Litestream or LiteFS for database replication
3. Update clients with all server URLs

See [docs/deployment/ha-setup.md](deployment/ha-setup.md) for details.

### Set Up Lighthouses

If the control plane acts as lighthouse:

1. Set `provide_lighthouse=true` in cluster config
2. Server will spawn Nebula processes automatically
3. Lighthouses restart when config version changes

### Configure Topology

```bash
# Assign lighthouse role
curl -X POST http://localhost:8080/api/v1/topology/lighthouse \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node-xxx",
    "public_ip": "1.2.3.4",
    "public_port": 4242
  }'

# Add node routes
curl -X PUT http://localhost:8080/api/v1/routes \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "routes": ["10.0.1.0/24", "10.0.2.0/24"]
  }'
```

### Monitor Your Deployment

```bash
# Health checks
curl http://localhost:8080/health/liveness
curl http://localhost:8080/health/readiness
curl http://localhost:8080/health/master

# Prometheus metrics (if enabled)
curl http://localhost:8080/metrics
```

### Rotate Tokens

```bash
# Rotate node token
curl -X POST http://localhost:8080/api/v1/nodes/{node_id}/token \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN"

# Rotate cluster token
curl -X POST http://localhost:8080/api/v1/tokens/cluster/rotate \
  -H "X-NebulaGC-Node-Token: $ADMIN_TOKEN"
```

## Common Issues

### "Authentication failed"

- Verify token is correct and hasn't been rotated
- Check token is at least 41 characters
- Ensure you're using the correct header (`X-NebulaGC-Node-Token` or `X-NebulaGC-Cluster-Token`)

### "Database is locked"

- Ensure only one master instance is running
- Check database file permissions
- Verify WAL mode is enabled

### "Connection refused"

- Verify server is running: `curl http://localhost:8080/health/liveness`
- Check firewall rules
- Verify listen address and port

## Additional Resources

- **[API Reference](api-reference.md)** - Complete API documentation
- **[Operations Manual](operations.md)** - Production deployment guide
- **[Architecture](architecture.md)** - System architecture overview
- **[Deployment Guides](deployment/)** - Systemd, Docker, Kubernetes
- **[GitHub Repository](https://github.com/yaroslav-gwit/nebulagc)** - Source code and issues

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/yaroslav-gwit/nebulagc/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yaroslav-gwit/nebulagc/discussions)
- **Documentation**: Browse the `docs/` directory

---

**Next**: [API Reference](api-reference.md) for complete endpoint documentation
