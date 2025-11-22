# LiteFS SQLite Replication Guide

This guide covers using LiteFS for distributed SQLite database replication across multiple nodes.

---

## Overview

[LiteFS](https://fly.io/docs/litefs/) is a FUSE-based file system for distributed SQLite databases. Key features:

- **Distributed Reads**: Read replicas across multiple nodes
- **Single Writer**: One primary node handles writes
- **Automatic Failover**: Automatic promotion of replicas
- **Strong Consistency**: Ensures data consistency across nodes
- **Cloud-Native**: Designed for container environments

LiteFS vs Litestream:

| Feature | Litestream | LiteFS |
|---------|-----------|---------|
| **Primary Use** | Disaster recovery | High availability |
| **Replication** | To object storage | To replica nodes |
| **Write Mode** | Single writer | Single writer |
| **Read Mode** | Restore required | Direct reads |
| **Latency** | Seconds (async) | Milliseconds (sync) |
| **Cost** | Storage costs | Compute costs |
| **Best For** | Backups, DR | HA, read scaling |

---

## How LiteFS Works

1. **FUSE Mount**: Mounts as virtual file system at `/litefs`
2. **Primary Election**: Consul-based leader election
3. **Transaction Replication**: Streams SQLite pages to replicas
4. **WAL Synchronization**: Keeps replicas in sync with primary
5. **Read Replicas**: Replicas serve read-only queries

---

## Architecture

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Node 1    │      │   Node 2    │      │   Node 3    │
│  (Primary)  │─────▶│  (Replica)  │      │  (Replica)  │
│             │      │             │      │             │
│ /litefs/    │      │ /litefs/    │      │ /litefs/    │
│  ├─ db      │      │  └─ db (RO) │      │  └─ db (RO) │
└─────────────┘      └─────────────┘      └─────────────┘
       │                    │                     │
       └────────────┬───────┴─────────────────────┘
                    │
              ┌─────▼─────┐
              │  Consul   │
              │ (Leader   │
              │ Election) │
              └───────────┘
```

---

## Installation

### Linux (Binary)

```bash
# Download latest release
wget https://github.com/superfly/litefs/releases/download/v0.5.10/litefs-v0.5.10-linux-amd64.tar.gz

# Extract
tar -xzf litefs-v0.5.10-linux-amd64.tar.gz

# Install
sudo mv litefs /usr/local/bin/
sudo chmod +x /usr/local/bin/litefs

# Verify
litefs version
```

### Docker

```bash
docker pull flyio/litefs:latest
```

### From Source

```bash
go install github.com/superfly/litefs/cmd/litefs@latest
```

---

## Configuration

### Basic Configuration

Create `/etc/litefs.yml`:

```yaml
# LiteFS mount point
fuse:
  dir: "/litefs"

# Data directory (where actual data is stored)
data:
  dir: "/var/lib/litefs"

# Lease configuration (Consul-based)
lease:
  type: "consul"
  advertise-url: "http://${HOSTNAME}:20202"
  candidate: true  # Allow this node to become primary
  hostname: "${HOSTNAME}"
  
  consul:
    url: "http://consul:8500"
    key: "litefs/nebulagc-primary"

# Proxy configuration (optional - for write forwarding)
proxy:
  addr: ":20202"
  target: "localhost:8080"
  db: "nebulagc.db"

# Replication to other nodes
http:
  addr: ":20202"
```

### Environment Variables

```bash
HOSTNAME=$(hostname)
LITEFS_DIR=/litefs
CONSUL_URL=http://consul:8500
```

---

## Deployment

### Consul Setup

LiteFS requires Consul for leader election:

```bash
# Run Consul in Docker
docker run -d \
  --name consul \
  -p 8500:8500 \
  -p 8600:8600/udp \
  consul:latest agent -server -ui -bootstrap-expect=1 -client=0.0.0.0
```

Or use managed Consul service (AWS, GCP, Azure).

### Primary Node

Create `/etc/systemd/system/litefs.service`:

```ini
[Unit]
Description=LiteFS Distributed SQLite File System
Documentation=https://fly.io/docs/litefs/
After=network-online.target
Wants=network-online.target
Before=nebulagc-server.service

[Service]
Type=simple
User=root
Group=root

# Environment
Environment=HOSTNAME=%H
Environment=CONSUL_URL=http://consul:8500

# LiteFS
ExecStart=/usr/local/bin/litefs mount -config /etc/litefs.yml

# Restart policy
Restart=always
RestartSec=5

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=litefs

[Install]
WantedBy=multi-user.target
```

Update nebulagc-server database path to use LiteFS mount:

```bash
# /etc/nebulagc/server.env
NEBULAGC_DB_PATH=/litefs/nebulagc.db
```

Update systemd dependency:

```ini
# /etc/systemd/system/nebulagc-server.service
[Unit]
After=litefs.service
Requires=litefs.service
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable litefs
sudo systemctl start litefs
sudo systemctl start nebulagc-server
```

### Replica Nodes

Same configuration, but set `candidate: true` in `litefs.yml` to allow failover.

---

## Docker Integration

### Dockerfile with LiteFS

```dockerfile
FROM flyio/litefs:0.5 AS litefs

FROM alpine:3.19

# Copy LiteFS binary
COPY --from=litefs /usr/local/bin/litefs /usr/local/bin/litefs

# Install dependencies
RUN apk add --no-cache ca-certificates fuse3

# Copy server binary
COPY nebulagc-server /usr/local/bin/

# Create directories
RUN mkdir -p /litefs /var/lib/litefs /etc/litefs

# Copy LiteFS config
COPY litefs.yml /etc/litefs.yml

# Entrypoint
COPY docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh

# Expose ports
EXPOSE 8080 20202

ENTRYPOINT ["/docker-entrypoint.sh"]
```

Entrypoint script:

```bash
#!/bin/sh
set -e

# Start LiteFS in background
litefs mount -config /etc/litefs.yml &
LITEFS_PID=$!

# Wait for LiteFS to be ready
echo "Waiting for LiteFS to mount..."
while [ ! -d "/litefs" ]; do
  sleep 1
done

# Start server
exec nebulagc-server
```

### Docker Compose with LiteFS

```yaml
version: '3.8'

services:
  consul:
    image: consul:latest
    command: agent -server -ui -bootstrap-expect=1 -client=0.0.0.0
    ports:
      - "8500:8500"
    networks:
      - litefs

  server-primary:
    build:
      context: .
      dockerfile: Dockerfile.litefs
    environment:
      HOSTNAME: server-primary
      CONSUL_URL: http://consul:8500
      NEBULAGC_DB_PATH: /litefs/nebulagc.db
      NEBULAGC_HA_MODE: master
    ports:
      - "8080:8080"
      - "20202:20202"
    depends_on:
      - consul
    cap_add:
      - SYS_ADMIN
    devices:
      - /dev/fuse
    security_opt:
      - apparmor:unconfined
    networks:
      - litefs

  server-replica1:
    build:
      context: .
      dockerfile: Dockerfile.litefs
    environment:
      HOSTNAME: server-replica1
      CONSUL_URL: http://consul:8500
      NEBULAGC_DB_PATH: /litefs/nebulagc.db
      NEBULAGC_HA_MODE: replica
      NEBULAGC_MASTER_URL: http://server-primary:8080
    ports:
      - "8081:8080"
      - "20203:20202"
    depends_on:
      - consul
      - server-primary
    cap_add:
      - SYS_ADMIN
    devices:
      - /dev/fuse
    security_opt:
      - apparmor:unconfined
    networks:
      - litefs

  server-replica2:
    build:
      context: .
      dockerfile: Dockerfile.litefs
    environment:
      HOSTNAME: server-replica2
      CONSUL_URL: http://consul:8500
      NEBULAGC_DB_PATH: /litefs/nebulagc.db
      NEBULAGC_HA_MODE: replica
      NEBULAGC_MASTER_URL: http://server-primary:8080
    ports:
      - "8082:8080"
      - "20204:20202"
    depends_on:
      - consul
      - server-primary
    cap_add:
      - SYS_ADMIN
    devices:
      - /dev/fuse
    security_opt:
      - apparmor:unconfined
    networks:
      - litefs

networks:
  litefs:
    driver: bridge
```

Start cluster:

```bash
docker-compose up -d
```

---

## Kubernetes Integration

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: litefs-config
  namespace: nebulagc
data:
  litefs.yml: |
    fuse:
      dir: "/litefs"
    
    data:
      dir: "/var/lib/litefs"
    
    lease:
      type: "consul"
      advertise-url: "http://${HOSTNAME}.nebulagc-server.nebulagc.svc.cluster.local:20202"
      candidate: true
      hostname: "${HOSTNAME}"
      
      consul:
        url: "http://consul:8500"
        key: "litefs/nebulagc-primary"
    
    proxy:
      addr: ":20202"
      target: "localhost:8080"
      db: "nebulagc.db"
    
    http:
      addr: ":20202"
```

### StatefulSet with LiteFS Sidecar

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nebulagc-server
  namespace: nebulagc
spec:
  serviceName: nebulagc-server
  replicas: 3
  template:
    spec:
      initContainers:
      # Initialize LiteFS directories
      - name: init-litefs
        image: alpine:3.19
        command:
        - sh
        - -c
        - |
          mkdir -p /litefs /var/lib/litefs
          chown -R 1000:1000 /litefs /var/lib/litefs
        volumeMounts:
        - name: litefs-data
          mountPath: /var/lib/litefs
        securityContext:
          runAsUser: 0
      
      containers:
      # LiteFS sidecar
      - name: litefs
        image: flyio/litefs:0.5
        args:
        - mount
        - -config
        - /etc/litefs/litefs.yml
        env:
        - name: HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        ports:
        - name: litefs
          containerPort: 20202
        volumeMounts:
        - name: litefs-config
          mountPath: /etc/litefs
        - name: litefs-data
          mountPath: /var/lib/litefs
        - name: litefs-mount
          mountPath: /litefs
          mountPropagation: Bidirectional
        securityContext:
          privileged: true  # Required for FUSE
          capabilities:
            add:
            - SYS_ADMIN
      
      # Server container
      - name: server
        image: nebulagc-server:latest
        env:
        - name: NEBULAGC_DB_PATH
          value: "/litefs/nebulagc.db"
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        volumeMounts:
        - name: litefs-mount
          mountPath: /litefs
        ports:
        - name: http
          containerPort: 8080
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
      
      volumes:
      - name: litefs-config
        configMap:
          name: litefs-config
      - name: litefs-mount
        emptyDir: {}
  
  volumeClaimTemplates:
  - metadata:
      name: litefs-data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

### Consul Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: consul
  namespace: nebulagc
spec:
  replicas: 1
  selector:
    matchLabels:
      app: consul
  template:
    metadata:
      labels:
        app: consul
    spec:
      containers:
      - name: consul
        image: consul:latest
        args:
        - agent
        - -server
        - -ui
        - -bootstrap-expect=1
        - -client=0.0.0.0
        ports:
        - containerPort: 8500
---
apiVersion: v1
kind: Service
metadata:
  name: consul
  namespace: nebulagc
spec:
  selector:
    app: consul
  ports:
  - name: http
    port: 8500
    targetPort: 8500
```

---

## Failover

### Automatic Failover

When primary fails:

1. **Consul detects failure** (missed heartbeats)
2. **Lease expires** (typically 10-30 seconds)
3. **New primary elected** from replicas
4. **Replicas connect** to new primary
5. **Writes resume** to new primary

### Manual Failover

```bash
# On old primary (if accessible)
litefs -config /etc/litefs.yml demote

# Force new leader election in Consul
consul kv delete litefs/nebulagc-primary
```

### Testing Failover

```bash
# Kill primary
docker stop server-primary

# Watch new leader election
docker logs server-replica1 -f

# Verify new primary
curl http://localhost:8081/health
```

---

## Monitoring

### Check Cluster Status

```bash
# View primary node
litefs -config /etc/litefs.yml status

# Output:
# Role: primary
# Database: nebulagc.db
# Position: 1234567
# Replicas:
#   - server-replica1 (lag: 0)
#   - server-replica2 (lag: 0)
```

### Consul Status

```bash
# Check leader
consul kv get litefs/nebulagc-primary

# Consul UI
open http://localhost:8500/ui
```

### Prometheus Metrics

LiteFS exposes metrics at `:20202/metrics`:

```bash
curl http://localhost:20202/metrics
```

Key metrics:
- `litefs_is_primary`: 1 if primary, 0 if replica
- `litefs_replication_lag_seconds`: Replication lag
- `litefs_connection_count`: Number of replicas
- `litefs_db_size_bytes`: Database size

### Grafana Dashboard

Example queries:

```promql
# Primary node
litefs_is_primary == 1

# Replication lag
litefs_replication_lag_seconds

# Database size
litefs_db_size_bytes / 1024 / 1024  # MB
```

---

## Performance Tuning

### Replication Lag

Typical lag: <10ms under normal load

Factors affecting lag:
- Network latency between nodes
- Write throughput
- Number of replicas
- Database size

### Write Performance

LiteFS adds minimal overhead:
- **Local writes**: ~1-2ms latency
- **Replication**: Async, doesn't block writes
- **Synchronous mode**: Available for stronger consistency

```yaml
# In litefs.yml
sync:
  mode: "full"  # Wait for all replicas (slower, stronger consistency)
  # mode: "normal"  # Default, async replication
```

### Read Performance

Reads are served locally from replicas:
- **Latency**: Same as local SQLite
- **Scalability**: Horizontal scaling with replicas
- **Consistency**: May lag primary by milliseconds

---

## Best Practices

1. **Use Consul Cluster**: Single Consul node is SPOF
   ```bash
   # 3-node Consul cluster
   consul agent -server -bootstrap-expect=3 -data-dir=/consul/data
   ```

2. **Configure Health Checks**: Monitor lease status
   ```yaml
   # In litefs.yml
   lease:
     ttl: "10s"
     renew-interval: "1s"
   ```

3. **Set Candidate Priority**: Control which nodes become primary
   ```yaml
   lease:
     candidate: true
     priority: 100  # Higher = more likely to be elected
   ```

4. **Monitor Replication Lag**: Alert on high lag
   ```bash
   if [ "$LAG" -gt 100 ]; then
     echo "WARNING: Replication lag is ${LAG}ms"
   fi
   ```

5. **Backup Primary Data**: LiteFS doesn't replace backups
   - Use Litestream alongside LiteFS
   - Regular snapshots to object storage

6. **Test Failover**: Regularly test failover scenarios
   ```bash
   # Chaos testing
   docker kill -s SIGKILL server-primary
   ```

7. **Resource Limits**: FUSE can be resource-intensive
   ```yaml
   resources:
     limits:
       cpu: 500m
       memory: 512Mi
   ```

8. **Security**: Secure Consul communication
   ```yaml
   consul:
     url: "https://consul:8501"
     ca-file: "/etc/ssl/certs/ca.pem"
     cert-file: "/etc/ssl/certs/client.pem"
     key-file: "/etc/ssl/private/client-key.pem"
   ```

---

## Troubleshooting

### LiteFS Won't Mount

```bash
# Check FUSE support
modprobe fuse
lsmod | grep fuse

# Check permissions
ls -ld /litefs

# Check logs
journalctl -u litefs -f

# Verify config
litefs -config /etc/litefs.yml mount -debug
```

### Replication Not Working

```bash
# Check connectivity between nodes
telnet server-replica1 20202

# Verify Consul connection
curl http://consul:8500/v1/kv/litefs/nebulagc-primary

# Check LiteFS status
litefs -config /etc/litefs.yml status
```

### High Replication Lag

```bash
# Check network latency
ping -c 10 server-replica1

# Monitor LiteFS metrics
curl http://localhost:20202/metrics | grep lag

# Check database size (large DBs = more lag)
du -h /litefs/nebulagc.db
```

### Failover Not Working

```bash
# Check Consul leader
consul operator raft list-peers

# Verify candidate nodes
consul kv get litefs/nebulagc-primary

# Check lease TTL
# In /etc/litefs.yml: lease.ttl should be reasonable (10-30s)
```

### Database Corruption

```bash
# Verify database integrity on primary
sqlite3 /litefs/nebulagc.db "PRAGMA integrity_check;"

# Force re-sync from primary (on replica)
systemctl stop litefs
rm -rf /var/lib/litefs/*
systemctl start litefs
```

---

## Migration from Litestream

To use both Litestream (backup) and LiteFS (HA):

```yaml
# /etc/litefs.yml
fuse:
  dir: "/litefs"

data:
  dir: "/var/lib/litefs"

# ... lease and proxy config ...

# Litestream integration (optional)
litestream:
  enabled: true
  config: "/etc/litestream.yml"
```

Update Litestream config:

```yaml
# /etc/litestream.yml
dbs:
  - path: /litefs/nebulagc.db  # Point to LiteFS mount
    replicas:
      - url: s3://my-bucket/nebulagc/db
```

Both services:
- **LiteFS**: Handles HA and read replicas
- **Litestream**: Handles disaster recovery backups

---

## When to Use LiteFS

**Use LiteFS when**:
- Need high availability (automatic failover)
- Want read replicas for scaling
- Low-latency replication required
- Running in Kubernetes or containers

**Use Litestream when**:
- Single-server deployment
- Need disaster recovery only
- Cost-sensitive (storage vs compute)
- Infrequent database access

**Use Both when**:
- Production deployments
- Need HA + DR
- Compliance requires off-site backups

---

## Additional Resources

- [LiteFS Official Documentation](https://fly.io/docs/litefs/)
- [LiteFS GitHub](https://github.com/superfly/litefs)
- [Consul Documentation](https://www.consul.io/docs)
- [FUSE Documentation](https://www.kernel.org/doc/html/latest/filesystems/fuse.html)
- [SQLite Replication Strategies](https://fly.io/blog/sqlite-replication/)
