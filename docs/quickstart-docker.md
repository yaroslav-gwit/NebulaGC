# Quick Start Guide - Docker Deployment

This guide will get you up and running with NebulaGC using Docker in under 10 minutes.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Server Deployment](#server-deployment)
- [Firewall Configuration](#firewall-configuration)
- [Admin Setup](#admin-setup)
- [Node Onboarding](#node-onboarding)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Server Requirements

- **OS**: Any OS with Docker support (Linux, macOS, Windows)
- **Docker**: Version 20.10+ installed
- **Docker Compose**: Version 2.0+ (optional but recommended)
- **RAM**: Minimum 512MB, recommended 2GB+
- **Disk**: Minimum 2GB free space
- **Network**: Static IP address recommended

### Client Requirements

- **OS**: Linux, macOS, or Windows with WSL2
- **Docker**: Version 20.10+ (optional, for containerized client)
- **Nebula**: Install Nebula binary from [github.com/slackhq/nebula](https://github.com/slackhq/nebula/releases)
- **Network**: Outbound internet access to reach NebulaGC server

### Tools Needed

```bash
# Install Docker (Ubuntu/Debian)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
newgrp docker

# Install Docker Compose
sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

# Verify installation
docker --version
docker-compose --version
```

## Server Deployment

### Option 1: Docker Compose (Recommended)

#### Step 1: Create Project Directory

```bash
# Create directory structure
mkdir -p ~/nebulagc-docker
cd ~/nebulagc-docker
mkdir -p data config
```

#### Step 2: Generate HMAC Secret

```bash
# Generate secret and save to file
openssl rand -base64 32 > config/hmac-secret
chmod 600 config/hmac-secret
```

**IMPORTANT**: Save this secret! You'll need it for backups and replica servers.

#### Step 3: Create Docker Compose File

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  nebulagc:
    image: nebulagc:latest  # Or yaroslavgwit/nebulagc:v1.0.0-rc
    container_name: nebulagc-server
    restart: unless-stopped
    
    ports:
      - "8443:8443"      # NebulaGC API
      - "4242:4242/udp"  # Nebula Lighthouse (optional)
    
    volumes:
      - ./data:/data
      - ./config:/config
      - ./lighthouses:/lighthouses
    
    environment:
      - NEBULAGC_DB_PATH=/data/nebulagc.db
      - NEBULAGC_LISTEN=:8443
      - NEBULAGC_HMAC_SECRET_FILE=/config/hmac-secret
      - NEBULAGC_HA_MODE=master
      - NEBULAGC_LOG_LEVEL=info
      - NEBULAGC_LOG_FORMAT=json
      - NEBULAGC_LIGHTHOUSE_DIR=/lighthouses
    
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8443/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    
    networks:
      - nebulagc-network

networks:
  nebulagc-network:
    driver: bridge

volumes:
  nebulagc-data:
  nebulagc-config:
  nebulagc-lighthouses:
```

#### Step 4: Build Docker Image (if not using pre-built)

If building from source:

```bash
# Clone repository
git clone https://github.com/yaroslav-gwit/nebulagc.git
cd nebulagc

# Build Docker image
docker build -t nebulagc:latest -f Dockerfile .

# Or use multi-stage build
docker build -t nebulagc:latest .

# Verify image
docker images | grep nebulagc
```

Create `Dockerfile` if not present:

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.work go.work
COPY models/go.mod models/go.mod
COPY pkg/go.mod pkg/go.mod
COPY sdk/go.mod sdk/go.mod
COPY server/go.mod server/go.mod

# Download dependencies
WORKDIR /build/server
RUN go mod download

# Copy source code
WORKDIR /build
COPY . .

# Build server
WORKDIR /build/server
RUN CGO_ENABLED=1 go build -o nebulagc-server ./cmd/nebulagc-server

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs curl

# Create user
RUN addgroup -g 1000 nebulagc && \
    adduser -D -u 1000 -G nebulagc nebulagc

# Create directories
RUN mkdir -p /data /config /lighthouses && \
    chown -R nebulagc:nebulagc /data /config /lighthouses

# Copy binary
COPY --from=builder /build/server/nebulagc-server /usr/local/bin/

# Switch to non-root user
USER nebulagc

# Expose ports
EXPOSE 8443

# Set working directory
WORKDIR /data

# Health check
HEALTHCHECK --interval=30s --timeout=10s --retries=3 \
  CMD curl -f http://localhost:8443/health || exit 1

# Run server
ENTRYPOINT ["/usr/local/bin/nebulagc-server"]
```

#### Step 5: Start NebulaGC Server

```bash
# Start services
cd ~/nebulagc-docker
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f nebulagc
```

**Expected output**:
```
nebulagc-server | {"level":"info","timestamp":"2025-11-22T10:00:00Z","msg":"Starting NebulaGC server","version":"1.0.0-rc"}
nebulagc-server | {"level":"info","timestamp":"2025-11-22T10:00:00Z","msg":"Server listening","address":":8443"}
```

#### Step 6: Verify Server is Running

```bash
# Check health
curl http://localhost:8443/health

# Check version
curl http://localhost:8443/version

# Check container status
docker ps | grep nebulagc
```

**Expected response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:00:00Z",
  "version": "1.0.0-rc",
  "ha_role": "master"
}
```

### Option 2: Docker Run (Simple Deployment)

For a simpler single-container deployment:

```bash
# Generate HMAC secret
HMAC_SECRET=$(openssl rand -base64 32)

# Create directories
mkdir -p ~/nebulagc-data

# Run container
docker run -d \
  --name nebulagc-server \
  --restart unless-stopped \
  -p 8443:8443 \
  -p 4242:4242/udp \
  -v ~/nebulagc-data:/data \
  -e NEBULAGC_DB_PATH=/data/nebulagc.db \
  -e NEBULAGC_LISTEN=:8443 \
  -e NEBULAGC_HMAC_SECRET="$HMAC_SECRET" \
  -e NEBULAGC_HA_MODE=master \
  -e NEBULAGC_LOG_LEVEL=info \
  -e NEBULAGC_LOG_FORMAT=json \
  nebulagc:latest

# Save the HMAC secret
echo "$HMAC_SECRET" > ~/nebulagc-data/hmac-secret.txt
chmod 600 ~/nebulagc-data/hmac-secret.txt

# Check logs
docker logs -f nebulagc-server

# Verify health
curl http://localhost:8443/health
```

## Firewall Configuration

### Server Firewall Rules

If using a host firewall (ufw/firewalld):

```bash
# NebulaGC API
sudo ufw allow 8443/tcp comment "NebulaGC API"

# Nebula Lighthouse (if enabled)
sudo ufw allow 4242/udp comment "Nebula Lighthouse"

# Enable firewall
sudo ufw enable
sudo ufw status
```

**Port Summary**:
- `8443/tcp` - NebulaGC REST API (required)
- `4242/udp` - Nebula Lighthouse port (optional, if server acts as lighthouse)

**Docker Networking Note**: Docker automatically manages iptables rules for published ports.

### Client Firewall Rules

For Docker-based clients:

```bash
# Allow outbound to NebulaGC server
sudo ufw allow out 8443/tcp comment "NebulaGC API"

# Allow Nebula P2P
sudo ufw allow 4242/udp comment "Nebula P2P"
```

**Client Port Summary**:
- `8443/tcp` (outbound) - NebulaGC API access
- `4242/udp` (bidirectional) - Nebula P2P communication

## Admin Setup

### Step 1: Access Container Database

```bash
# Access database via container
docker exec -it nebulagc-server sqlite3 /data/nebulagc.db

# Or with docker-compose
docker-compose exec nebulagc sqlite3 /data/nebulagc.db
```

### Step 2: Create Tenant and Cluster

```sql
-- Create tenant
INSERT INTO tenants (id, name, description, created_at, updated_at)
VALUES (
  '550e8400-e29b-41d4-a716-446655440000',
  'my-company',
  'Production Nebula Network',
  datetime('now'),
  datetime('now')
);

-- Create cluster
INSERT INTO clusters (id, tenant_id, name, description, created_at, updated_at)
VALUES (
  '660e8400-e29b-41d4-a716-446655440001',
  '550e8400-e29b-41d4-a716-446655440000',
  'production',
  'Production cluster',
  datetime('now'),
  datetime('now')
);

-- Verify
SELECT * FROM tenants;
SELECT * FROM clusters;

-- Exit
.quit
```

### Step 3: Create Admin Node

```bash
# Get your server's public IP
SERVER_IP=$(curl -s ifconfig.me)
echo "Server IP: $SERVER_IP"

# Create admin node
curl -X POST http://localhost:8443/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "admin-node",
    "is_lighthouse": false,
    "overlay_ip": "10.42.0.254/16"
  }'
```

**Save the auth_token from the response**:
```bash
# Export token
export NEBULAGC_ADMIN_TOKEN="token_from_response"

# Or save to file
echo "token_from_response" > ~/nebulagc-admin-token
chmod 600 ~/nebulagc-admin-token
```

### Step 4: Create Lighthouse Node

```bash
# Create lighthouse node
curl -X POST http://localhost:8443/api/v1/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -d "{
    \"cluster_id\": \"660e8400-e29b-41d4-a716-446655440001\",
    \"name\": \"lighthouse-01\",
    \"is_lighthouse\": true,
    \"overlay_ip\": \"10.42.0.1/16\",
    \"public_ip\": \"$SERVER_IP\"
  }"
```

### Step 5: Prepare and Upload Config Bundle

```bash
# Create bundle directory on host
mkdir -p ~/nebula-bundle
cd ~/nebula-bundle

# Generate CA (or copy existing)
nebula-cert ca -name "My Company Production"

# Create config template
cat > config.yml <<'EOF'
pki:
  ca: /etc/nebula/ca.crt
  cert: /etc/nebula/host.crt
  key: /etc/nebula/host.key

lighthouse:
  am_lighthouse: false
  interval: 60
  hosts:
    - "10.42.0.1"

listen:
  host: 0.0.0.0
  port: 4242

punchy:
  punch: true
  respond: true

tun:
  disabled: false
  dev: nebula1
  drop_local_broadcast: false
  drop_multicast: false
  tx_queue: 500
  mtu: 1300

logging:
  level: info
  format: text

firewall:
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
    - port: 22
      proto: tcp
      host: any
EOF

# Create tarball
tar czf bundle.tar.gz ca.crt config.yml

# Upload to NebulaGC
curl -X POST "http://localhost:8443/api/v1/bundles/660e8400-e29b-41d4-a716-446655440001" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Bundle-Version: 1.0.0" \
  --data-binary @bundle.tar.gz
```

### Step 6: Configure Topology

```bash
# Set topology
curl -X POST "http://localhost:8443/api/v1/topology/660e8400-e29b-41d4-a716-446655440001" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -d "{
    \"nodes\": [
      {
        \"node_id\": \"880e8400-e29b-41d4-a716-446655440003\",
        \"is_lighthouse\": true,
        \"overlay_ip\": \"10.42.0.1\",
        \"public_ip\": \"$SERVER_IP\",
        \"port\": 4242
      }
    ]
  }"
```

## Node Onboarding

### Step 1: Create Client Node

```bash
# Create client node
curl -X POST http://localhost:8443/api/v1/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -d '{
    "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "client-node-01",
    "is_lighthouse": false,
    "overlay_ip": "10.42.0.10/16"
  }'
```

**Save the auth_token**:
```bash
export CLIENT_TOKEN="token_from_response"
```

### Step 2: Option A - Native Nebula on Client

On the client machine:

```bash
# Set variables
export NEBULAGC_SERVER="http://YOUR_SERVER_IP:8443"
export CLUSTER_ID="660e8400-e29b-41d4-a716-446655440001"

# Download bundle
curl -o nebula-bundle.tar.gz \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  "$NEBULAGC_SERVER/api/v1/bundles/$CLUSTER_ID/latest"

# Extract
sudo mkdir -p /etc/nebula
cd /etc/nebula
sudo tar xzf ~/nebula-bundle.tar.gz

# Generate node certificate (on CA machine)
nebula-cert sign \
  -name "client-node-01" \
  -ip "10.42.0.10/16" \
  -ca-crt ca.crt \
  -ca-key ca.key

# Copy certificates to client
sudo cp client-node-01.crt /etc/nebula/host.crt
sudo cp client-node-01.key /etc/nebula/host.key
sudo chmod 600 /etc/nebula/host.key

# Install Nebula binary
wget https://github.com/slackhq/nebula/releases/download/v1.8.0/nebula-linux-amd64.tar.gz
tar xzf nebula-linux-amd64.tar.gz
sudo cp nebula /usr/local/bin/
sudo chmod +x /usr/local/bin/nebula

# Run Nebula
sudo nebula -config /etc/nebula/config.yml
```

### Step 2: Option B - Docker Nebula on Client

```bash
# Download bundle
curl -o nebula-bundle.tar.gz \
  -H "Authorization: Bearer $CLIENT_TOKEN" \
  "http://YOUR_SERVER_IP:8443/api/v1/bundles/$CLUSTER_ID/latest"

# Create directory
mkdir -p ~/nebula-client
cd ~/nebula-client
tar xzf ~/nebula-bundle.tar.gz

# Generate node certificate
nebula-cert sign \
  -name "client-node-01" \
  -ip "10.42.0.10/16" \
  -ca-crt ca.crt \
  -ca-key ca.key

cp client-node-01.crt host.crt
cp client-node-01.key host.key

# Create Docker Compose file
cat > docker-compose.yml <<'EOF'
version: '3.8'

services:
  nebula:
    image: slackhq/nebula:latest
    container_name: nebula-client
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
    volumes:
      - ./ca.crt:/etc/nebula/ca.crt:ro
      - ./host.crt:/etc/nebula/host.crt:ro
      - ./host.key:/etc/nebula/host.key:ro
      - ./config.yml:/etc/nebula/config.yml:ro
    command: -config /etc/nebula/config.yml
    network_mode: host

EOF

# Start Nebula
docker-compose up -d

# Check logs
docker-compose logs -f nebula
```

### Step 3: Verify Client Connection

```bash
# Check interface (native)
ip addr show nebula1

# Or check container (Docker)
docker exec nebula-client ip addr show nebula1

# Ping lighthouse
ping -c 4 10.42.0.1

# Check Nebula logs
# Native:
sudo journalctl -u nebula -f

# Docker:
docker logs -f nebula-client
```

## Verification

### Test 1: Check NebulaGC Container

```bash
# Check container status
docker ps | grep nebulagc

# Check container logs
docker logs nebulagc-server --tail 50

# Check container health
docker inspect nebulagc-server | jq '.[0].State.Health'
```

### Test 2: Check API Endpoints

```bash
# Health check
curl http://localhost:8443/health

# List nodes
curl -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  "http://localhost:8443/api/v1/nodes?cluster_id=660e8400-e29b-41d4-a716-446655440001"

# Get topology
curl -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  "http://localhost:8443/api/v1/topology/660e8400-e29b-41d4-a716-446655440001"
```

### Test 3: Check Nebula Connectivity

```bash
# From client, ping lighthouse
ping -c 4 10.42.0.1

# Check Nebula interface
ip addr show nebula1
```

### Test 4: Check Metrics

```bash
# Prometheus metrics
curl http://localhost:8443/metrics | grep nebulagc_nodes
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker logs nebulagc-server

# Check container inspect
docker inspect nebulagc-server

# Common issues:
# 1. Port conflicts
docker ps -a | grep 8443
sudo netstat -tlnp | grep 8443

# 2. Volume permission issues
ls -la ~/nebulagc-docker/data
docker exec nebulagc-server ls -la /data

# 3. Missing HMAC secret
docker exec nebulagc-server cat /config/hmac-secret
```

### Database Issues

```bash
# Access database
docker exec -it nebulagc-server sqlite3 /data/nebulagc.db

# Check integrity
PRAGMA integrity_check;

# List tables
.tables

# Exit
.quit
```

### Network Connectivity Issues

```bash
# Check Docker networks
docker network ls
docker network inspect nebulagc-network

# Test connectivity to container
docker exec nebulagc-server ping -c 2 google.com

# Check published ports
docker port nebulagc-server
```

### Client Connection Issues

```bash
# Test API connectivity
telnet YOUR_SERVER_IP 8443

# Check client token
curl -v -H "Authorization: Bearer $CLIENT_TOKEN" \
  "http://YOUR_SERVER_IP:8443/api/v1/nodes"

# Check Nebula logs
docker logs nebula-client

# Test UDP connectivity
sudo tcpdump -i any udp port 4242
```

### Backup and Restore

```bash
# Backup database
docker exec nebulagc-server sqlite3 /data/nebulagc.db ".backup '/data/backup.db'"
docker cp nebulagc-server:/data/backup.db ~/nebulagc-backup-$(date +%Y%m%d).db

# Restore database
docker cp ~/nebulagc-backup-20251122.db nebulagc-server:/data/nebulagc.db
docker restart nebulagc-server
```

## Docker Maintenance

### Update NebulaGC Image

```bash
# Pull latest image
docker pull nebulagc:latest

# Stop and remove old container
docker-compose down

# Start with new image
docker-compose up -d

# Or without compose:
docker stop nebulagc-server
docker rm nebulagc-server
# Run docker run command again with new image
```

### View Logs

```bash
# Follow logs
docker-compose logs -f nebulagc

# Last 100 lines
docker logs nebulagc-server --tail 100

# With timestamps
docker logs nebulagc-server -t
```

### Container Management

```bash
# Stop container
docker-compose stop

# Start container
docker-compose start

# Restart container
docker-compose restart

# Remove containers and volumes
docker-compose down -v
```

## Next Steps

1. **Add More Nodes**: Repeat node onboarding for additional clients
2. **Configure HA**: Deploy replica containers with shared storage
3. **Enable TLS**: Add TLS termination with Nginx/Traefik
4. **Set Up Monitoring**: Configure Prometheus to scrape metrics
5. **Automate Backups**: Schedule database backup jobs

## Additional Resources

- [Docker Documentation](https://docs.docker.com/)
- [Architecture Overview](architecture.md)
- [API Reference](api-reference.md)
- [Operations Manual](operations.md)

---

**Congratulations! Your NebulaGC control plane is running in Docker with your first node connected!** 🎉
