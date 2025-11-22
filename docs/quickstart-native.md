# Quick Start Guide - Native Deployment

This guide will get you up and running with NebulaGC using native binaries in under 10 minutes.

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

- **OS**: Linux (Ubuntu 20.04+, RHEL 8+, Debian 11+) or macOS
- **Go**: 1.22+ (for building from source)
- **RAM**: Minimum 512MB, recommended 2GB+
- **Disk**: Minimum 1GB free space
- **Network**: Static IP address recommended

### Client Requirements

- **OS**: Linux or macOS
- **Nebula**: Install Nebula binary from [github.com/slackhq/nebula](https://github.com/slackhq/nebula/releases)
- **Network**: Outbound internet access to reach NebulaGC server

### Tools Needed

```bash
# Install required tools
sudo apt-get update
sudo apt-get install -y curl jq sqlite3

# For building from source
sudo apt-get install -y git golang-1.22
```

## Server Deployment

### Step 1: Build NebulaGC Server

```bash
# Clone repository
git clone https://github.com/yaroslav-gwit/nebulagc.git
cd nebulagc

# Build server binary
make build-server

# Verify build
./bin/nebulagc-server --version
```

**Expected output**:
```
NebulaGC Server v1.0.0-rc
Build: abc123def
Go: go1.22.0
```

### Step 2: Install Server Binary

```bash
# Copy binary to system path
sudo cp bin/nebulagc-server /usr/local/bin/
sudo chmod +x /usr/local/bin/nebulagc-server

# Create nebulagc user
sudo useradd -r -s /bin/false -d /var/lib/nebulagc nebulagc

# Create directories
sudo mkdir -p /var/lib/nebulagc
sudo mkdir -p /etc/nebulagc
sudo mkdir -p /var/log/nebulagc

# Set permissions
sudo chown -R nebulagc:nebulagc /var/lib/nebulagc
sudo chown -R nebulagc:nebulagc /var/log/nebulagc
sudo chmod 750 /var/lib/nebulagc
sudo chmod 750 /etc/nebulagc
```

### Step 3: Generate HMAC Secret

```bash
# Generate cryptographically secure secret
openssl rand -base64 32 | sudo tee /etc/nebulagc/hmac-secret

# Secure the secret file
sudo chmod 600 /etc/nebulagc/hmac-secret
sudo chown nebulagc:nebulagc /etc/nebulagc/hmac-secret
```

**IMPORTANT**: Save this secret! You'll need it if you restore from backup or add replica servers.

### Step 4: Create systemd Service

Create `/etc/systemd/system/nebulagc.service`:

```bash
sudo tee /etc/systemd/system/nebulagc.service > /dev/null <<'EOF'
[Unit]
Description=NebulaGC Control Plane Server
Documentation=https://github.com/yaroslav-gwit/nebulagc
After=network.target

[Service]
Type=simple
User=nebulagc
Group=nebulagc
WorkingDirectory=/var/lib/nebulagc

# Server configuration
Environment="NEBULAGC_DB_PATH=/var/lib/nebulagc/nebulagc.db"
Environment="NEBULAGC_LISTEN=:8443"
Environment="NEBULAGC_HMAC_SECRET_FILE=/etc/nebulagc/hmac-secret"
Environment="NEBULAGC_HA_MODE=master"
Environment="NEBULAGC_LOG_LEVEL=info"
Environment="NEBULAGC_LOG_FORMAT=json"
Environment="NEBULAGC_LIGHTHOUSE_DIR=/var/lib/nebulagc/lighthouses"

ExecStart=/usr/local/bin/nebulagc-server

# Restart configuration
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nebulagc /var/log/nebulagc

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nebulagc

[Install]
WantedBy=multi-user.target
EOF
```

### Step 5: Start NebulaGC Server

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (start on boot)
sudo systemctl enable nebulagc

# Start service
sudo systemctl start nebulagc

# Check status
sudo systemctl status nebulagc
```

**Expected output**:
```
● nebulagc.service - NebulaGC Control Plane Server
   Loaded: loaded (/etc/systemd/system/nebulagc.service; enabled)
   Active: active (running) since Thu 2025-11-22 10:00:00 UTC
```

### Step 6: Verify Server is Running

```bash
# Check health endpoint
curl http://localhost:8443/health

# Check version
curl http://localhost:8443/version
```

**Expected responses**:

```json
// Health response
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:00:00Z",
  "version": "1.0.0-rc",
  "ha_role": "master"
}

// Version response
{
  "version": "1.0.0-rc",
  "commit": "abc123def",
  "build_date": "2025-11-22T09:00:00Z",
  "go_version": "go1.22.0"
}
```

## Firewall Configuration

### Server Firewall Rules

Open these ports on the NebulaGC server:

```bash
# NebulaGC API (HTTPS recommended in production)
sudo ufw allow 8443/tcp comment "NebulaGC API"

# Lighthouse UDP port (if server acts as lighthouse)
sudo ufw allow 4242/udp comment "Nebula Lighthouse"

# Enable firewall
sudo ufw enable
sudo ufw status
```

**Port Summary**:
- `8443/tcp` - NebulaGC REST API (required)
- `4242/udp` - Nebula Lighthouse port (if lighthouse enabled)
- Port 4242 can be different per cluster (configured in topology)

**Production Recommendation**:
- Use HTTPS (TLS) with valid certificates
- Restrict API access to known IP ranges
- Use HAProxy/Nginx for TLS termination

### Client Firewall Rules

Open these ports on NebulaGC client nodes:

```bash
# Outbound to NebulaGC server
sudo ufw allow out 8443/tcp comment "NebulaGC API"

# Outbound to Nebula lighthouse
sudo ufw allow out 4242/udp comment "Nebula Lighthouse"

# Nebula P2P communication (inbound)
sudo ufw allow 4242/udp comment "Nebula P2P"
```

**Client Port Summary**:
- `8443/tcp` (outbound) - NebulaGC API access
- `4242/udp` (outbound) - Connect to lighthouse
- `4242/udp` (inbound) - Accept Nebula P2P connections
- Nebula P2P port configured in node's config.yml

**Note**: If using strict egress rules, whitelist the NebulaGC server IP.

## Admin Setup

### Step 1: Create Tenant and Cluster

Currently, tenant and cluster creation is done via direct database access. CLI commands will be added in a future release.

```bash
# Access database
sudo -u nebulagc sqlite3 /var/lib/nebulagc/nebulagc.db

# Create tenant
INSERT INTO tenants (id, name, description, created_at, updated_at)
VALUES (
  '550e8400-e29b-41d4-a716-446655440000',
  'my-company',
  'Production Nebula Network',
  datetime('now'),
  datetime('now')
);

# Create cluster
INSERT INTO clusters (id, tenant_id, name, description, created_at, updated_at)
VALUES (
  '660e8400-e29b-41d4-a716-446655440001',
  '550e8400-e29b-41d4-a716-446655440000',
  'production',
  'Production cluster',
  datetime('now'),
  datetime('now')
);

# Verify
SELECT * FROM tenants;
SELECT * FROM clusters;

# Exit
.quit
```

**Expected output**:
```
550e8400-e29b-41d4-a716-446655440000|my-company|Production Nebula Network|2025-11-22 10:00:00|2025-11-22 10:00:00
660e8400-e29b-41d4-a716-446655440001|550e8400-e29b-41d4-a716-446655440000|production|Production cluster|2025-11-22 10:00:00|2025-11-22 10:00:00
```

### Step 2: Create Admin Node

Create an administrative node for API access:

```bash
# Create admin node via API (using bootstrap mode)
# Note: In production, use proper admin authentication
curl -X POST http://localhost:8443/api/v1/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "admin-node",
    "is_lighthouse": false,
    "overlay_ip": "10.42.0.254/16"
  }'
```

**Expected response**:
```json
{
  "id": "770e8400-e29b-41d4-a716-446655440002",
  "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "admin-node",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.254/16",
  "public_ip": "",
  "auth_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "created_at": "2025-11-22T10:00:00Z",
  "updated_at": "2025-11-22T10:00:00Z"
}
```

**IMPORTANT**: Save the `auth_token`! This is the only time it will be shown. You'll use it for all API requests.

```bash
# Save token to environment variable
export NEBULAGC_ADMIN_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

# Or save to file
echo "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." > ~/.nebulagc-admin-token
chmod 600 ~/.nebulagc-admin-token
```

### Step 3: Create First Lighthouse Node

```bash
# Create lighthouse node
curl -X POST http://localhost:8443/api/v1/nodes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -d '{
    "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "lighthouse-01",
    "is_lighthouse": true,
    "overlay_ip": "10.42.0.1/16",
    "public_ip": "203.0.113.10"
  }'
```

**Response**:
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440003",
  "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "lighthouse-01",
  "is_lighthouse": true,
  "overlay_ip": "10.42.0.1/16",
  "public_ip": "203.0.113.10",
  "auth_token": "lighthouse_token_here...",
  "created_at": "2025-11-22T10:00:00Z",
  "updated_at": "2025-11-22T10:00:00Z"
}
```

**Note**: Replace `203.0.113.10` with your server's actual public IP address.

### Step 4: Prepare Nebula Configuration Bundle

Create a configuration bundle with Nebula CA certificate and config template:

```bash
# Create bundle directory
mkdir -p /tmp/nebula-bundle
cd /tmp/nebula-bundle

# Generate CA (if you don't have one)
nebula-cert ca -name "My Company Production"

# This creates:
# - ca.crt (CA certificate)
# - ca.key (CA private key - keep secure!)

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

# Download nebula-lighthouse binary (optional)
# wget https://github.com/slackhq/nebula/releases/download/v1.8.0/nebula-linux-amd64.tar.gz
# tar xzf nebula-linux-amd64.tar.gz
# mv nebula nebula-lighthouse

# Create tarball
tar czf bundle.tar.gz ca.crt config.yml
# If including nebula binary: tar czf bundle.tar.gz ca.crt config.yml nebula-lighthouse
```

### Step 5: Upload Configuration Bundle

```bash
# Upload bundle to NebulaGC
curl -X POST "http://localhost:8443/api/v1/bundles/660e8400-e29b-41d4-a716-446655440001" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -H "Content-Type: application/octet-stream" \
  -H "X-Bundle-Version: 1.0.0" \
  --data-binary @bundle.tar.gz
```

**Expected response**:
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440004",
  "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
  "version": "1.0.0",
  "hash": "sha256:abc123def456...",
  "size_bytes": 8192,
  "created_at": "2025-11-22T10:00:00Z"
}
```

### Step 6: Define Network Topology

```bash
# Set topology (lighthouse configuration)
curl -X POST "http://localhost:8443/api/v1/topology/660e8400-e29b-41d4-a716-446655440001" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  -d '{
    "nodes": [
      {
        "node_id": "880e8400-e29b-41d4-a716-446655440003",
        "is_lighthouse": true,
        "overlay_ip": "10.42.0.1",
        "public_ip": "203.0.113.10",
        "port": 4242
      }
    ]
  }'
```

**Expected response**:
```json
{
  "message": "Topology updated successfully",
  "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
  "nodes_updated": 1,
  "lighthouses": 1
}
```

## Node Onboarding

### Step 1: Create Client Node

On the NebulaGC server, create a node for your client:

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

**Save the response** including the auth token:
```json
{
  "id": "aa0e8400-e29b-41d4-a716-446655440005",
  "cluster_id": "660e8400-e29b-41d4-a716-446655440001",
  "name": "client-node-01",
  "is_lighthouse": false,
  "overlay_ip": "10.42.0.10/16",
  "auth_token": "client_node_token_here...",
  "created_at": "2025-11-22T10:00:00Z",
  "updated_at": "2025-11-22T10:00:00Z"
}
```

### Step 2: Download Config Bundle on Client

On the client machine:

```bash
# Set environment variables
export NEBULAGC_SERVER="http://203.0.113.10:8443"
export NEBULAGC_TOKEN="client_node_token_here..."
export CLUSTER_ID="660e8400-e29b-41d4-a716-446655440001"

# Download bundle
curl -o nebula-bundle.tar.gz \
  -H "Authorization: Bearer $NEBULAGC_TOKEN" \
  "$NEBULAGC_SERVER/api/v1/bundles/$CLUSTER_ID/latest"

# Extract bundle
sudo mkdir -p /etc/nebula
cd /etc/nebula
sudo tar xzf ~/nebula-bundle.tar.gz

# Verify files
ls -la /etc/nebula/
```

**Expected files**:
```
-rw-r--r-- 1 root root 1234 Nov 22 10:00 ca.crt
-rw-r--r-- 1 root root 2345 Nov 22 10:00 config.yml
```

### Step 3: Generate Node Certificate

Generate a certificate for the client node:

```bash
# On a machine with the CA key (keep CA key secure!)
nebula-cert sign \
  -name "client-node-01" \
  -ip "10.42.0.10/16" \
  -ca-crt /path/to/ca.crt \
  -ca-key /path/to/ca.key

# This creates:
# - client-node-01.crt
# - client-node-01.key

# Copy to client machine
scp client-node-01.crt client-node-01.key user@client:/tmp/
```

On the client machine:

```bash
# Move certificates
sudo mv /tmp/client-node-01.crt /etc/nebula/host.crt
sudo mv /tmp/client-node-01.key /etc/nebula/host.key

# Secure permissions
sudo chmod 600 /etc/nebula/host.key
sudo chmod 644 /etc/nebula/host.crt
sudo chmod 644 /etc/nebula/ca.crt
sudo chmod 644 /etc/nebula/config.yml
```

### Step 4: Install Nebula Binary

```bash
# Download Nebula binary
wget https://github.com/slackhq/nebula/releases/download/v1.8.0/nebula-linux-amd64.tar.gz
tar xzf nebula-linux-amd64.tar.gz

# Install binary
sudo cp nebula /usr/local/bin/
sudo chmod +x /usr/local/bin/nebula

# Verify installation
nebula -version
```

### Step 5: Create Nebula Service

Create `/etc/systemd/system/nebula.service`:

```bash
sudo tee /etc/systemd/system/nebula.service > /dev/null <<'EOF'
[Unit]
Description=Nebula Overlay Network
Documentation=https://github.com/slackhq/nebula
After=network.target
Wants=network-online.target

[Service]
Type=simple
Restart=on-failure
RestartSec=5s
ExecStart=/usr/local/bin/nebula -config /etc/nebula/config.yml
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF
```

### Step 6: Start Nebula Service

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service
sudo systemctl enable nebula

# Start service
sudo systemctl start nebula

# Check status
sudo systemctl status nebula
```

**Expected output**:
```
● nebula.service - Nebula Overlay Network
   Loaded: loaded (/etc/systemd/system/nebula.service; enabled)
   Active: active (running) since Thu 2025-11-22 10:00:00 UTC
```

### Step 7: Verify Client Logs

```bash
# Check Nebula logs
sudo journalctl -u nebula -f

# Look for successful connection
# Expected log messages:
# - "Lighthouse responded with update"
# - "Handshake completed"
```

## Verification

### Test 1: Check Node Status

On the server:

```bash
# List all nodes
curl -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  "http://localhost:8443/api/v1/nodes?cluster_id=660e8400-e29b-41d4-a716-446655440001"
```

**Expected**: All nodes listed with their status.

### Test 2: Check Nebula Interface

On the client:

```bash
# Check interface is up
ip addr show nebula1

# Expected output:
# nebula1: <POINTOPOINT,MULTICAST,NOARP,UP,LOWER_UP> mtu 1300
#     inet 10.42.0.10/16 scope global nebula1
```

### Test 3: Ping Lighthouse

From the client node:

```bash
# Ping lighthouse through Nebula
ping -c 4 10.42.0.1
```

**Expected**: Successful ping responses.

### Test 4: Check Nebula Status

```bash
# Check Nebula status (if using control socket)
sudo nebula -config /etc/nebula/config.yml -print-tunnels
```

### Test 5: Monitor Metrics

Check Prometheus metrics:

```bash
# On server
curl http://localhost:8443/metrics | grep nebulagc
```

**Expected metrics**:
```
nebulagc_nodes_total{cluster="production"} 2
nebulagc_lighthouse_processes 1
nebulagc_api_requests_total{endpoint="/api/v1/nodes",method="POST",status="201"} 2
```

## Troubleshooting

### Server Not Starting

**Problem**: Service fails to start

```bash
# Check logs
sudo journalctl -u nebulagc -n 50

# Common issues:
# 1. Port already in use
sudo netstat -tlnp | grep 8443

# 2. Permission issues
sudo ls -la /var/lib/nebulagc
sudo ls -la /etc/nebulagc/hmac-secret

# 3. Database corruption
sudo -u nebulagc sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA integrity_check;"
```

### Client Cannot Connect

**Problem**: Client cannot reach NebulaGC API

```bash
# Test connectivity
telnet 203.0.113.10 8443

# Check firewall
sudo ufw status

# Check server logs
sudo journalctl -u nebulagc -f
```

### Nebula Not Starting

**Problem**: Nebula service fails to start

```bash
# Check configuration
sudo nebula -config /etc/nebula/config.yml -test

# Check logs
sudo journalctl -u nebula -n 50

# Common issues:
# 1. Certificate issues
ls -la /etc/nebula/
sudo nebula-cert print -path /etc/nebula/host.crt

# 2. Configuration errors
sudo cat /etc/nebula/config.yml | grep -E "ca|cert|key"
```

### Lighthouse Connection Fails

**Problem**: Client cannot connect to lighthouse

```bash
# Verify lighthouse is running on server
sudo systemctl status nebulagc
sudo journalctl -u nebulagc | grep lighthouse

# Check topology configuration
curl -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN" \
  "http://localhost:8443/api/v1/topology/660e8400-e29b-41d4-a716-446655440001"

# Test UDP connectivity
sudo tcpdump -i any udp port 4242
```

### Authentication Errors

**Problem**: API returns 401 Unauthorized

```bash
# Verify token is correct
echo $NEBULAGC_ADMIN_TOKEN

# Check token in database
sudo -u nebulagc sqlite3 /var/lib/nebulagc/nebulagc.db \
  "SELECT id, name, auth_token FROM nodes WHERE name = 'admin-node';"

# Regenerate token if needed
curl -X POST "http://localhost:8443/api/v1/nodes/770e8400-e29b-41d4-a716-446655440002/token" \
  -H "Authorization: Bearer $NEBULAGC_ADMIN_TOKEN"
```

## Next Steps

1. **Add More Nodes**: Repeat the node onboarding process for additional clients
2. **Configure HA**: Set up replica servers for high availability
3. **Enable Monitoring**: Configure Prometheus and Grafana dashboards
4. **Set Up Backups**: Implement automated database backups
5. **Configure TLS**: Enable HTTPS with valid certificates

## Additional Resources

- [Architecture Overview](architecture.md)
- [API Reference](api-reference.md)
- [Operations Manual](operations.md)
- [Troubleshooting Guide](operations.md#troubleshooting)

---

**Congratulations! Your NebulaGC control plane is now running with your first node connected!** 🎉
