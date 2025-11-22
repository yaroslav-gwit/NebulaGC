# NebulaGC Operations Manual

Complete operational guide for deploying, monitoring, and maintaining NebulaGC in production.

## Table of Contents

- [Deployment](#deployment)
- [Configuration](#configuration)
- [Monitoring](#monitoring)
- [Backup and Recovery](#backup-and-recovery)
- [Troubleshooting](#troubleshooting)
- [Maintenance](#maintenance)
- [Security Operations](#security-operations)
- [Performance Tuning](#performance-tuning)
- [Disaster Recovery](#disaster-recovery)

## Deployment

### Single Instance Deployment

Simplest deployment for development or small-scale production.

#### systemd Service

1. **Install binary**:

```bash
sudo cp bin/nebulagc-server /usr/local/bin/
sudo chmod +x /usr/local/bin/nebulagc-server
```

2. **Create service file** `/etc/systemd/system/nebulagc.service`:

```ini
[Unit]
Description=NebulaGC Control Plane Server
After=network.target

[Service]
Type=simple
User=nebulagc
Group=nebulagc
WorkingDirectory=/var/lib/nebulagc
ExecStart=/usr/local/bin/nebulagc-server \
  --db-path=/var/lib/nebulagc/nebulagc.db \
  --listen=:8080 \
  --hmac-secret-file=/etc/nebulagc/hmac-secret
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

3. **Create user and directories**:

```bash
sudo useradd -r -s /bin/false nebulagc
sudo mkdir -p /var/lib/nebulagc /etc/nebulagc
sudo chown nebulagc:nebulagc /var/lib/nebulagc
```

4. **Generate HMAC secret**:

```bash
openssl rand -base64 32 | sudo tee /etc/nebulagc/hmac-secret
sudo chmod 600 /etc/nebulagc/hmac-secret
sudo chown nebulagc:nebulagc /etc/nebulagc/hmac-secret
```

5. **Start service**:

```bash
sudo systemctl daemon-reload
sudo systemctl enable nebulagc
sudo systemctl start nebulagc
sudo systemctl status nebulagc
```

#### Docker Deployment

1. **Create Dockerfile** (if not using pre-built image):

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /build
COPY . .
RUN make build-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /build/bin/nebulagc-server /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/nebulagc-server"]
```

2. **Build image**:

```bash
docker build -t nebulagc:latest .
```

3. **Run container**:

```bash
docker run -d \
  --name nebulagc \
  -p 8080:8080 \
  -v /var/lib/nebulagc:/data \
  -e NEBULAGC_DB_PATH=/data/nebulagc.db \
  -e NEBULAGC_HMAC_SECRET=$(openssl rand -base64 32) \
  --restart unless-stopped \
  nebulagc:latest
```

4. **Use Docker Compose**:

```yaml
version: '3.8'
services:
  nebulagc:
    image: nebulagc:latest
    ports:
      - "8080:8080"
    volumes:
      - nebulagc-data:/data
    environment:
      - NEBULAGC_DB_PATH=/data/nebulagc.db
      - NEBULAGC_HMAC_SECRET_FILE=/run/secrets/hmac-secret
    secrets:
      - hmac-secret
    restart: unless-stopped

volumes:
  nebulagc-data:

secrets:
  hmac-secret:
    file: ./hmac-secret.txt
```

```bash
docker-compose up -d
```

### High Availability Deployment

Production-ready HA setup with master and replicas.

#### Architecture

```
        Load Balancer (HAProxy/Nginx)
               │
    ┌──────────┼──────────┐
    │          │          │
  Master    Replica1   Replica2
    │          │          │
    └──────────┴──────────┘
         Shared Storage
      (or DB Replication)
```

#### HAProxy Configuration

```haproxy
global
    log /dev/log local0
    maxconn 4096

defaults
    log global
    mode http
    option httplog
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms

frontend nebulagc_frontend
    bind *:443 ssl crt /etc/ssl/certs/nebulagc.pem
    default_backend nebulagc_backend

backend nebulagc_backend
    balance roundrobin
    option httpchk GET /health
    http-check expect status 200
    
    server master 10.0.1.10:8080 check inter 5s fall 3 rise 2
    server replica1 10.0.1.11:8080 check inter 5s fall 3 rise 2 backup
    server replica2 10.0.1.12:8080 check inter 5s fall 3 rise 2 backup
```

#### Master Configuration

```bash
# /etc/nebulagc/master.env
NEBULAGC_HA_MODE=master
NEBULAGC_DB_PATH=/var/lib/nebulagc/nebulagc.db
NEBULAGC_LISTEN=:8080
NEBULAGC_HMAC_SECRET_FILE=/etc/nebulagc/hmac-secret
NEBULAGC_LOG_LEVEL=info
NEBULAGC_LOG_FORMAT=json
```

#### Replica Configuration

```bash
# /etc/nebulagc/replica1.env
NEBULAGC_HA_MODE=replica
NEBULAGC_HA_MASTER_URL=http://10.0.1.10:8080
NEBULAGC_DB_PATH=/var/lib/nebulagc/nebulagc.db
NEBULAGC_LISTEN=:8080
NEBULAGC_HMAC_SECRET_FILE=/etc/nebulagc/hmac-secret
NEBULAGC_LOG_LEVEL=info
NEBULAGC_LOG_FORMAT=json
```

#### Database Replication (Litestream)

1. **Install Litestream** on all instances:

```bash
wget https://github.com/benbjohnson/litestream/releases/download/v0.3.9/litestream-v0.3.9-linux-amd64.tar.gz
tar xzf litestream-v0.3.9-linux-amd64.tar.gz
sudo mv litestream /usr/local/bin/
```

2. **Configure Litestream** `/etc/litestream.yml`:

```yaml
dbs:
  - path: /var/lib/nebulagc/nebulagc.db
    replicas:
      - url: s3://nebulagc-backups/nebulagc-prod
        sync-interval: 1s
        retention: 168h  # 7 days
```

3. **Start Litestream**:

```bash
litestream replicate -config /etc/litestream.yml
```

#### Kubernetes Deployment

1. **Create namespace**:

```bash
kubectl create namespace nebulagc
```

2. **Create secret for HMAC**:

```bash
kubectl create secret generic nebulagc-hmac \
  --from-literal=secret=$(openssl rand -base64 32) \
  -n nebulagc
```

3. **Deploy StatefulSet** `k8s/statefulset.yaml`:

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nebulagc
  namespace: nebulagc
spec:
  serviceName: nebulagc
  replicas: 3
  selector:
    matchLabels:
      app: nebulagc
  template:
    metadata:
      labels:
        app: nebulagc
    spec:
      containers:
      - name: nebulagc
        image: nebulagc:0.9.0
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: NEBULAGC_DB_PATH
          value: /data/nebulagc.db
        - name: NEBULAGC_HMAC_SECRET
          valueFrom:
            secretKeyRef:
              name: nebulagc-hmac
              key: secret
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NEBULAGC_HA_MODE
          value: "$(if [ \"$(POD_NAME)\" = \"nebulagc-0\" ]; then echo master; else echo replica; fi)"
        volumeMounts:
        - name: data
          mountPath: /data
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 10Gi
```

4. **Create service**:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nebulagc
  namespace: nebulagc
spec:
  type: LoadBalancer
  selector:
    app: nebulagc
  ports:
  - port: 443
    targetPort: 8080
    protocol: TCP
```

5. **Apply configuration**:

```bash
kubectl apply -f k8s/
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NEBULAGC_DB_PATH` | SQLite database file path | `./nebulagc.db` | No |
| `NEBULAGC_LISTEN` | Server listen address | `:8080` | No |
| `NEBULAGC_HMAC_SECRET` | HMAC secret for tokens | - | Yes |
| `NEBULAGC_HMAC_SECRET_FILE` | Path to HMAC secret file | - | Alt to HMAC_SECRET |
| `NEBULAGC_HA_MODE` | HA mode (master/replica) | `master` | No |
| `NEBULAGC_HA_MASTER_URL` | Master URL for replicas | - | If replica |
| `NEBULAGC_LOG_LEVEL` | Log level (debug/info/warn/error) | `info` | No |
| `NEBULAGC_LOG_FORMAT` | Log format (json/console) | `console` | No |
| `NEBULAGC_LIGHTHOUSE_DIR` | Lighthouse working directory | `/tmp/lighthouses` | No |

### Configuration File (Future)

Future versions will support YAML configuration:

```yaml
# nebulagc.yaml
database:
  path: /var/lib/nebulagc/nebulagc.db
  max_connections: 10

server:
  listen: :8080
  tls:
    enabled: true
    cert_file: /etc/nebulagc/tls.crt
    key_file: /etc/nebulagc/tls.key

ha:
  mode: master
  heartbeat_interval: 10s
  failover_timeout: 30s

lighthouse:
  working_dir: /var/lib/nebulagc/lighthouses
  port_range: "4242-4342"

logging:
  level: info
  format: json
  output: /var/log/nebulagc/server.log

security:
  hmac_secret_file: /etc/nebulagc/hmac-secret
  rate_limit:
    global: 1000
    per_ip: 100
    per_node: 20
```

## Monitoring

### Health Checks

**Endpoint**: `GET /health`

**Response**:
```json
{
  "status": "healthy",
  "timestamp": "2025-11-22T10:30:45Z",
  "version": "0.9.0",
  "ha_role": "master"
}
```

**Monitoring Script**:

```bash
#!/bin/bash
# /usr/local/bin/nebulagc-healthcheck.sh

ENDPOINT="http://localhost:8080/health"
TIMEOUT=5

response=$(curl -s -w "\n%{http_code}" --max-time $TIMEOUT "$ENDPOINT")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | head -n-1)

if [ "$http_code" = "200" ]; then
    status=$(echo "$body" | jq -r '.status')
    if [ "$status" = "healthy" ]; then
        echo "OK: Service is healthy"
        exit 0
    fi
fi

echo "CRITICAL: Service is unhealthy (HTTP $http_code)"
exit 2
```

### Prometheus Metrics

**Endpoint**: `GET /metrics`

**Key Metrics**:

```prometheus
# API Request Metrics
nebulagc_api_requests_total{endpoint="/api/v1/nodes",method="GET",status="200"} 1523
nebulagc_api_request_duration_seconds_bucket{endpoint="/api/v1/nodes",le="0.005"} 1420
nebulagc_api_request_duration_seconds_sum{endpoint="/api/v1/nodes"} 7.23
nebulagc_api_request_duration_seconds_count{endpoint="/api/v1/nodes"} 1523

# HA Metrics
nebulagc_ha_role{role="master"} 1
nebulagc_ha_failovers_total 2
nebulagc_ha_heartbeat_failures_total 0

# Resource Metrics
nebulagc_nodes_total{cluster="cluster-1"} 45
nebulagc_bundles_total{cluster="cluster-1"} 3
nebulagc_lighthouse_processes 2

# Database Metrics
nebulagc_db_connections_open 5
nebulagc_db_connections_max 10
nebulagc_db_query_duration_seconds_sum 1.23
```

**Prometheus Config** `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'nebulagc'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

**Alerting Rules** `alerts.yml`:

```yaml
groups:
  - name: nebulagc
    rules:
      - alert: NebulaGCDown
        expr: up{job="nebulagc"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "NebulaGC instance down"
          description: "{{ $labels.instance }} has been down for more than 1 minute"

      - alert: NebulaGCHighErrorRate
        expr: rate(nebulagc_api_requests_total{status=~"5.."}[5m]) > 0.05
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate on NebulaGC API"
          description: "Error rate is {{ $value }} errors/sec"

      - alert: NebulaGCSlowRequests
        expr: histogram_quantile(0.95, rate(nebulagc_api_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Slow API requests on NebulaGC"
          description: "95th percentile latency is {{ $value }}s"
```

### Grafana Dashboard

**Dashboard JSON** (excerpt):

```json
{
  "dashboard": {
    "title": "NebulaGC Monitoring",
    "panels": [
      {
        "title": "API Request Rate",
        "targets": [
          {
            "expr": "rate(nebulagc_api_requests_total[5m])"
          }
        ]
      },
      {
        "title": "API Latency (p95)",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(nebulagc_api_request_duration_seconds_bucket[5m]))"
          }
        ]
      },
      {
        "title": "HA Role",
        "targets": [
          {
            "expr": "nebulagc_ha_role"
          }
        ]
      }
    ]
  }
}
```

### Log Aggregation

**Fluentd Configuration**:

```conf
<source>
  @type tail
  path /var/log/nebulagc/*.log
  pos_file /var/log/td-agent/nebulagc.pos
  tag nebulagc
  <parse>
    @type json
    time_key timestamp
    time_format %Y-%m-%dT%H:%M:%S%z
  </parse>
</source>

<match nebulagc>
  @type elasticsearch
  host elasticsearch.local
  port 9200
  index_name nebulagc-logs
  type_name _doc
</match>
```

## Backup and Recovery

### Database Backup

#### Manual Backup

```bash
# Stop writes (or use WAL mode for online backup)
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA wal_checkpoint(FULL);"

# Create backup
cp /var/lib/nebulagc/nebulagc.db /backups/nebulagc-$(date +%Y%m%d-%H%M%S).db

# Verify backup
sqlite3 /backups/nebulagc-*.db "PRAGMA integrity_check;"
```

#### Automated Backup Script

```bash
#!/bin/bash
# /usr/local/bin/nebulagc-backup.sh

DB_PATH="/var/lib/nebulagc/nebulagc.db"
BACKUP_DIR="/backups/nebulagc"
RETENTION_DAYS=7

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Checkpoint WAL
sqlite3 "$DB_PATH" "PRAGMA wal_checkpoint(TRUNCATE);"

# Create backup with timestamp
BACKUP_FILE="$BACKUP_DIR/nebulagc-$(date +%Y%m%d-%H%M%S).db"
cp "$DB_PATH" "$BACKUP_FILE"

# Verify backup
if sqlite3 "$BACKUP_FILE" "PRAGMA integrity_check;" | grep -q "ok"; then
    echo "Backup successful: $BACKUP_FILE"
else
    echo "Backup verification failed!"
    exit 1
fi

# Remove old backups
find "$BACKUP_DIR" -name "nebulagc-*.db" -mtime +$RETENTION_DAYS -delete

# Optional: Upload to S3
# aws s3 cp "$BACKUP_FILE" s3://nebulagc-backups/$(basename "$BACKUP_FILE")
```

#### Cron Schedule

```cron
# Daily backup at 2 AM
0 2 * * * /usr/local/bin/nebulagc-backup.sh >> /var/log/nebulagc/backup.log 2>&1
```

### Database Restore

```bash
# Stop service
sudo systemctl stop nebulagc

# Restore from backup
cp /backups/nebulagc-20251122-020000.db /var/lib/nebulagc/nebulagc.db

# Fix permissions
sudo chown nebulagc:nebulagc /var/lib/nebulagc/nebulagc.db
sudo chmod 640 /var/lib/nebulagc/nebulagc.db

# Verify database
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA integrity_check;"

# Start service
sudo systemctl start nebulagc
```

## Troubleshooting

### Common Issues

#### Service Won't Start

**Symptoms**: systemd service fails to start

**Diagnosis**:

```bash
# Check service status
sudo systemctl status nebulagc

# Check logs
sudo journalctl -u nebulagc -n 100

# Check binary
/usr/local/bin/nebulagc-server --version

# Check permissions
ls -la /var/lib/nebulagc
```

**Common Causes**:
- Missing HMAC secret file
- Database file permissions incorrect
- Port already in use
- Binary missing or corrupted

**Solutions**:

```bash
# Generate HMAC secret if missing
openssl rand -base64 32 | sudo tee /etc/nebulagc/hmac-secret
sudo chmod 600 /etc/nebulagc/hmac-secret
sudo chown nebulagc:nebulagc /etc/nebulagc/hmac-secret

# Fix database permissions
sudo chown nebulagc:nebulagc /var/lib/nebulagc/nebulagc.db
sudo chmod 640 /var/lib/nebulagc/nebulagc.db

# Check port usage
sudo netstat -tlnp | grep 8080
# Kill conflicting process if needed
```

#### Authentication Failures

**Symptoms**: API requests return 401 Unauthorized

**Diagnosis**:

```bash
# Test with curl
curl -v http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer YOUR_TOKEN"

# Check token in database
sqlite3 /var/lib/nebulagc/nebulagc.db \
  "SELECT id, name, auth_token FROM nodes WHERE id = 'node-uuid';"
```

**Common Causes**:
- Token not included in request
- Token expired or invalid
- Token hashing mismatch (HMAC secret changed)
- Node deleted

**Solutions**:

```bash
# Regenerate token
curl -X POST http://localhost:8080/api/v1/nodes/node-uuid/token \
  -H "Authorization: Bearer admin-token"

# Verify HMAC secret is consistent
cat /etc/nebulagc/hmac-secret
```

#### High CPU Usage

**Symptoms**: Server consuming excessive CPU

**Diagnosis**:

```bash
# Check CPU usage
top -p $(pgrep nebulagc-server)

# Check goroutines
curl http://localhost:8080/debug/pprof/goroutine?debug=1

# Profile CPU
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

**Common Causes**:
- High request rate
- Inefficient database queries
- Goroutine leak
- Lighthouse process issues

**Solutions**:

```bash
# Check request rate
curl http://localhost:8080/metrics | grep nebulagc_api_requests_total

# Reduce rate limits if needed
# Edit /etc/systemd/system/nebulagc.service
# Add: Environment="NEBULAGC_RATE_LIMIT=500"

# Restart service
sudo systemctl daemon-reload
sudo systemctl restart nebulagc
```

#### Database Locked

**Symptoms**: "database is locked" errors

**Diagnosis**:

```bash
# Check WAL mode
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA journal_mode;"

# Check active connections
lsof /var/lib/nebulagc/nebulagc.db
```

**Solutions**:

```bash
# Enable WAL mode if not enabled
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA journal_mode=WAL;"

# Checkpoint and truncate WAL
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Restart service
sudo systemctl restart nebulagc
```

## Maintenance

### Routine Maintenance Tasks

#### Daily

- Monitor service health
- Review error logs
- Check disk space
- Verify backups completed

```bash
#!/bin/bash
# Daily health check script

echo "=== NebulaGC Daily Health Check ==="
echo "Date: $(date)"

# Check service status
if systemctl is-active --quiet nebulagc; then
    echo "✓ Service is running"
else
    echo "✗ Service is not running"
fi

# Check API health
if curl -sf http://localhost:8080/health > /dev/null; then
    echo "✓ API is responding"
else
    echo "✗ API is not responding"
fi

# Check disk space
DISK_USAGE=$(df -h /var/lib/nebulagc | tail -n1 | awk '{print $5}' | tr -d '%')
if [ "$DISK_USAGE" -lt 80 ]; then
    echo "✓ Disk usage: ${DISK_USAGE}%"
else
    echo "✗ Disk usage high: ${DISK_USAGE}%"
fi

# Check backup
LATEST_BACKUP=$(ls -t /backups/nebulagc/*.db 2>/dev/null | head -n1)
if [ -n "$LATEST_BACKUP" ]; then
    BACKUP_AGE=$(( ($(date +%s) - $(stat -f %m "$LATEST_BACKUP")) / 3600 ))
    if [ "$BACKUP_AGE" -lt 25 ]; then
        echo "✓ Latest backup: ${BACKUP_AGE}h ago"
    else
        echo "✗ Latest backup too old: ${BACKUP_AGE}h ago"
    fi
else
    echo "✗ No backups found"
fi
```

#### Weekly

- Review metrics and performance
- Database optimization (VACUUM, ANALYZE)
- Update dependencies
- Review and rotate logs

```bash
# Database optimization
sqlite3 /var/lib/nebulagc/nebulagc.db "VACUUM; ANALYZE;"

# Log rotation
sudo journalctl --vacuum-time=7d
```

#### Monthly

- Security updates
- Capacity planning review
- Disaster recovery drill
- Documentation review

### Database Optimization

```bash
# Analyze query performance
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA optimize;"

# Check database size
ls -lh /var/lib/nebulagc/nebulagc.db

# Check fragmentation
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA freelist_count;"

# Compact database (requires downtime)
sudo systemctl stop nebulagc
sqlite3 /var/lib/nebulagc/nebulagc.db "VACUUM;"
sudo systemctl start nebulagc
```

## Security Operations

### HMAC Secret Rotation

```bash
# Generate new secret
NEW_SECRET=$(openssl rand -base64 32)

# Update secret file
echo "$NEW_SECRET" | sudo tee /etc/nebulagc/hmac-secret
sudo chmod 600 /etc/nebulagc/hmac-secret

# Restart service (will regenerate all token hashes)
sudo systemctl restart nebulagc

# Regenerate all node tokens
for node_id in $(sqlite3 /var/lib/nebulagc/nebulagc.db "SELECT id FROM nodes;"); do
    curl -X POST http://localhost:8080/api/v1/nodes/$node_id/token \
      -H "Authorization: Bearer admin-token"
done
```

### TLS Certificate Renewal

```bash
# Let's Encrypt renewal (certbot)
sudo certbot renew

# Restart service to pick up new certs
sudo systemctl restart nebulagc

# Or use HAProxy/Nginx for TLS termination (recommended)
```

### Security Audit

```bash
# Check file permissions
sudo find /var/lib/nebulagc -type f -ls
sudo find /etc/nebulagc -type f -ls

# Review active tokens
sqlite3 /var/lib/nebulagc/nebulagc.db \
  "SELECT id, name, last_seen_at FROM nodes ORDER BY last_seen_at DESC;"

# Check for old, unused nodes
sqlite3 /var/lib/nebulagc/nebulagc.db \
  "SELECT id, name, last_seen_at FROM nodes WHERE last_seen_at < datetime('now', '-30 days');"
```

## Performance Tuning

### Database Tuning

```sql
-- Enable WAL mode for better concurrency
PRAGMA journal_mode = WAL;

-- Increase cache size (in pages, default 2000)
PRAGMA cache_size = -64000;  -- 64MB

-- Enable memory-mapped I/O (faster reads)
PRAGMA mmap_size = 30000000000;  -- 30GB

-- Synchronous = NORMAL (faster, still safe with WAL)
PRAGMA synchronous = NORMAL;

-- Optimize query planner
PRAGMA optimize;
```

### OS Tuning

```bash
# Increase file descriptor limits
echo "nebulagc soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "nebulagc hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# Kernel parameters for networking
sudo sysctl -w net.core.somaxconn=4096
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=8192

# Make permanent
echo "net.core.somaxconn=4096" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog=8192" | sudo tee -a /etc/sysctl.conf
```

## Disaster Recovery

### Full System Recovery

1. **Restore from backup**:

```bash
# Copy backup to new system
scp backup-server:/backups/nebulagc/nebulagc-latest.db /tmp/

# Install NebulaGC
sudo cp nebulagc-server /usr/local/bin/

# Restore database
sudo mkdir -p /var/lib/nebulagc
sudo cp /tmp/nebulagc-latest.db /var/lib/nebulagc/nebulagc.db

# Restore HMAC secret (from secure storage!)
sudo mkdir -p /etc/nebulagc
echo "YOUR_HMAC_SECRET" | sudo tee /etc/nebulagc/hmac-secret
sudo chmod 600 /etc/nebulagc/hmac-secret
```

2. **Verify and start**:

```bash
# Verify database integrity
sqlite3 /var/lib/nebulagc/nebulagc.db "PRAGMA integrity_check;"

# Start service
sudo systemctl start nebulagc

# Verify service
curl http://localhost:8080/health
```

3. **Verify data**:

```bash
# Check node count
sqlite3 /var/lib/nebulagc/nebulagc.db "SELECT COUNT(*) FROM nodes;"

# Test API
curl http://localhost:8080/api/v1/nodes \
  -H "Authorization: Bearer test-token"
```

### Multi-Region Failover

For geo-distributed deployments:

1. **Detect primary region failure**
2. **Promote replica in secondary region to master**
3. **Update DNS/load balancer to point to new master**
4. **Verify lighthouse processes started**
5. **Monitor for split-brain scenarios**

## Support

For additional help:

- **Documentation**: [Getting Started](getting-started.md) | [Architecture](architecture.md)
- **Issues**: https://github.com/yaroslav-gwit/nebulagc/issues
- **Discussions**: https://github.com/yaroslav-gwit/nebulagc/discussions

## Appendix

### Useful Commands

```bash
# Check service logs (last 100 lines)
sudo journalctl -u nebulagc -n 100

# Follow logs in real-time
sudo journalctl -u nebulagc -f

# Check database size
du -h /var/lib/nebulagc/nebulagc.db

# List all nodes
sqlite3 /var/lib/nebulagc/nebulagc.db "SELECT * FROM nodes;"

# Check rate limits
curl http://localhost:8080/metrics | grep rate_limit
```

### Emergency Contacts

Maintain an emergency contact list:

```markdown
## Emergency Contacts

- **On-Call Engineer**: +1-555-0123
- **Database Admin**: +1-555-0456
- **Security Team**: security@example.com
- **Infrastructure Team**: infra@example.com
```
