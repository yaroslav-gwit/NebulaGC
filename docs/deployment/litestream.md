# Litestream SQLite Replication Guide

This guide covers using Litestream for continuous SQLite database replication and disaster recovery.

---

## Overview

[Litestream](https://litestream.io/) provides streaming replication of SQLite databases to object storage (S3, GCS, Azure Blob Storage). Key benefits:

- **Continuous Backup**: Real-time replication of database changes
- **Point-in-Time Recovery**: Restore to any moment in time
- **Low Overhead**: Minimal impact on database performance
- **Cloud-Native**: Native support for major cloud providers
- **Cost-Effective**: Pay only for storage used

Litestream is ideal for:
- Disaster recovery
- Geographic redundancy
- Database migrations
- Compliance and audit requirements

---

## How Litestream Works

1. **WAL Monitoring**: Monitors SQLite WAL (Write-Ahead Log) file
2. **Snapshot + Incremental**: Takes periodic snapshots and streams WAL frames
3. **Cloud Upload**: Continuously uploads changes to object storage
4. **Restoration**: Can restore database from snapshots + WAL segments

---

## Installation

### Linux (Binary)

```bash
# Download latest release
wget https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz

# Extract
tar -xzf litestream-v0.3.13-linux-amd64.tar.gz

# Install
sudo mv litestream /usr/local/bin/
sudo chmod +x /usr/local/bin/litestream

# Verify
litestream version
```

### macOS (Homebrew)

```bash
brew install litestream
```

### Docker

```bash
docker pull litestream/litestream:latest
```

### From Source

```bash
go install github.com/benbjohnson/litestream/cmd/litestream@latest
```

---

## Configuration

### AWS S3

Create `/etc/litestream.yml`:

```yaml
# AWS S3 Configuration
access-key-id: ${AWS_ACCESS_KEY_ID}
secret-access-key: ${AWS_SECRET_ACCESS_KEY}

dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - url: s3://my-bucket/nebulagc/db
        region: us-east-1
        # Optional: Enable server-side encryption
        # sync-interval: 1s  # How often to check for new WAL frames (default: 1s)
        # retention: 24h     # How long to keep snapshots (default: 24h)
        # retention-check-interval: 1h  # How often to check for old snapshots
        # snapshot-interval: 1h  # How often to take full snapshots (default: 1h)
        # validation-interval: 6h  # How often to validate replicas
```

### Google Cloud Storage

```yaml
dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - url: gcs://my-bucket/nebulagc/db
        # Authentication via GOOGLE_APPLICATION_CREDENTIALS environment variable
```

### Azure Blob Storage

```yaml
dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - url: abs://my-account/my-container/nebulagc/db
        # Requires AZURE_STORAGE_ACCOUNT and AZURE_STORAGE_KEY environment variables
```

### MinIO / S3-Compatible

```yaml
dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - url: s3://my-bucket/nebulagc/db
        endpoint: https://minio.example.com:9000
        region: us-east-1
        force-path-style: true  # Required for MinIO
```

### Local File System (for testing)

```yaml
dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - path: /backup/nebulagc/db
```

---

## Running Litestream

### Standalone Process

```bash
# Run with config file
litestream replicate -config /etc/litestream.yml

# Run in foreground (for testing)
litestream replicate -config /etc/litestream.yml -exec "nebulagc-server"
```

### Systemd Service

Create `/etc/systemd/system/litestream.service`:

```ini
[Unit]
Description=Litestream SQLite Replication
Documentation=https://litestream.io/
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nebulagc-server
Group=nebulagc-server

# Environment
EnvironmentFile=/etc/litestream/env

# Litestream configuration
ExecStart=/usr/local/bin/litestream replicate -config /etc/litestream.yml

# Restart policy
Restart=always
RestartSec=5

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nebulagc-server
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=litestream

[Install]
WantedBy=multi-user.target
```

Environment file `/etc/litestream/env`:

```bash
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
AWS_REGION=us-east-1
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable litestream
sudo systemctl start litestream
sudo systemctl status litestream
```

### With nebulagc-server (Exec Mode)

Run Litestream as the parent process:

```bash
litestream replicate -config /etc/litestream.yml -exec "nebulagc-server"
```

Modify systemd service:

```ini
[Service]
ExecStart=/usr/local/bin/litestream replicate -config /etc/litestream.yml -exec "/usr/local/bin/nebulagc-server"
```

Litestream will:
1. Start replication
2. Launch nebulagc-server as child process
3. Continue replication while server runs
4. Shut down gracefully when server stops

---

## Docker Integration

### Dockerfile with Litestream

```dockerfile
FROM alpine:3.19

# Install Litestream
RUN wget https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64-static.tar.gz && \
    tar -xzf litestream-v0.3.13-linux-amd64-static.tar.gz -C /usr/local/bin && \
    rm litestream-v0.3.13-linux-amd64-static.tar.gz

# Copy server binary
COPY nebulagc-server /usr/local/bin/

# Copy Litestream config
COPY litestream.yml /etc/litestream.yml

# Entrypoint script
COPY docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh

ENTRYPOINT ["/docker-entrypoint.sh"]
```

Entrypoint script:

```bash
#!/bin/sh
set -e

# Restore database if it doesn't exist
if [ ! -f "/var/lib/nebulagc-server/nebulagc.db" ]; then
  echo "Restoring database from replica..."
  litestream restore -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db
fi

# Run Litestream with server
exec litestream replicate -config /etc/litestream.yml -exec "/usr/local/bin/nebulagc-server"
```

### Docker Compose with Litestream

```yaml
version: '3.8'

services:
  server:
    build:
      context: .
      dockerfile: Dockerfile.litestream
    environment:
      AWS_ACCESS_KEY_ID: ${AWS_ACCESS_KEY_ID}
      AWS_SECRET_ACCESS_KEY: ${AWS_SECRET_ACCESS_KEY}
      NEBULAGC_HMAC_SECRET: ${NEBULAGC_HMAC_SECRET}
    volumes:
      - server-data:/var/lib/nebulagc-server
    ports:
      - "8080:8080"

volumes:
  server-data:
```

---

## Kubernetes Integration

### Sidecar Container

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: nebulagc-server
  namespace: nebulagc
spec:
  replicas: 1
  serviceName: nebulagc-server
  template:
    spec:
      containers:
      # Main server container
      - name: server
        image: nebulagc-server:latest
        volumeMounts:
        - name: data
          mountPath: /var/lib/nebulagc-server
      
      # Litestream sidecar
      - name: litestream
        image: litestream/litestream:0.3.13
        args:
        - replicate
        - -config
        - /etc/litestream.yml
        env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: litestream-secret
              key: aws-access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: litestream-secret
              key: aws-secret-access-key
        volumeMounts:
        - name: data
          mountPath: /var/lib/nebulagc-server
        - name: litestream-config
          mountPath: /etc/litestream.yml
          subPath: litestream.yml
      
      volumes:
      - name: litestream-config
        configMap:
          name: litestream-config
  
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 10Gi
```

ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: litestream-config
  namespace: nebulagc
data:
  litestream.yml: |
    dbs:
      - path: /var/lib/nebulagc-server/nebulagc.db
        replicas:
          - url: s3://my-bucket/nebulagc/db
            region: us-east-1
```

Secret:

```bash
kubectl create secret generic litestream-secret \
  --from-literal=aws-access-key-id="${AWS_ACCESS_KEY_ID}" \
  --from-literal=aws-secret-access-key="${AWS_SECRET_ACCESS_KEY}" \
  --namespace=nebulagc
```

---

## Restoration

### Restore Latest

```bash
# Restore to latest point
litestream restore -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db

# Restore from specific replica
litestream restore -config /etc/litestream.yml -replica s3 /var/lib/nebulagc-server/nebulagc.db
```

### Point-in-Time Restore

```bash
# Restore to specific timestamp
litestream restore -config /etc/litestream.yml \
  -timestamp "2024-11-20T10:30:00Z" \
  /var/lib/nebulagc-server/nebulagc.db

# Restore to specific generation (after schema change)
litestream restore -config /etc/litestream.yml \
  -generation 1234567890abcdef \
  /var/lib/nebulagc-server/nebulagc.db
```

### List Available Snapshots

```bash
# Show all available snapshots and generations
litestream snapshots -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db

# Output:
# replica  generation        index  size      created
# s3       1234567890abcdef  0      10485760  2024-11-20T10:00:00Z
# s3       1234567890abcdef  1      10485760  2024-11-20T11:00:00Z
```

### Automated Restore on Startup

Script `/usr/local/bin/nebulagc-restore.sh`:

```bash
#!/bin/bash
set -e

DB_PATH="/var/lib/nebulagc-server/nebulagc.db"
CONFIG="/etc/litestream.yml"

# Check if database exists
if [ ! -f "$DB_PATH" ]; then
  echo "Database not found. Restoring from replica..."
  
  # Restore latest
  litestream restore -config "$CONFIG" "$DB_PATH"
  
  if [ $? -eq 0 ]; then
    echo "Database restored successfully"
  else
    echo "Failed to restore database"
    exit 1
  fi
else
  echo "Database already exists, skipping restore"
fi

# Set proper permissions
chown nebulagc-server:nebulagc-server "$DB_PATH"
chmod 600 "$DB_PATH"
```

---

## Monitoring

### Check Replication Status

```bash
# View replication position
litestream databases -config /etc/litestream.yml

# Output:
# path                                     replicas
# /var/lib/nebulagc-server/nebulagc.db    s3

# Detailed replica info
litestream replicas -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db

# Output:
# replica  generation        lag
# s3       1234567890abcdef  0s
```

### Prometheus Metrics

Litestream doesn't expose Prometheus metrics natively. Monitor via:

1. **Systemd Journal Logs**: Check for errors
2. **S3 Bucket Size**: Monitor storage usage
3. **WAL Lag**: Check `litestream replicas` output

Custom metrics exporter (example):

```bash
#!/bin/bash
# /usr/local/bin/litestream-exporter.sh

DB_PATH="/var/lib/nebulagc-server/nebulagc.db"
CONFIG="/etc/litestream.yml"

# Get lag from litestream
LAG=$(litestream replicas -config "$CONFIG" "$DB_PATH" 2>/dev/null | tail -n1 | awk '{print $3}' | sed 's/s//')

# Export to node_exporter textfile collector
echo "litestream_wal_lag_seconds{database=\"nebulagc\"} ${LAG:-999}" > /var/lib/node_exporter/textfile_collector/litestream.prom
```

Add to crontab:

```bash
* * * * * /usr/local/bin/litestream-exporter.sh
```

---

## Replication Lag

Litestream replication is **near real-time** with minimal lag:

- **Typical Lag**: <1 second under normal load
- **Sync Interval**: 1 second (configurable)
- **Factors Affecting Lag**:
  - Network latency to cloud storage
  - Write volume to database
  - Cloud storage API rate limits

### Measuring Lag

```bash
# Check current lag
litestream replicas -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db

# Monitor lag over time
watch -n 1 'litestream replicas -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db'
```

### Reducing Lag

```yaml
dbs:
  - path: /var/lib/nebulagc-server/nebulagc.db
    replicas:
      - url: s3://my-bucket/nebulagc/db
        sync-interval: 500ms  # More frequent checks (default: 1s)
```

**Trade-off**: Lower interval = more API calls = higher costs

---

## Disaster Recovery

### Scenario 1: Server Failure

```bash
# 1. Stop failed server
sudo systemctl stop nebulagc-server
sudo systemctl stop litestream

# 2. Remove corrupted database
sudo rm /var/lib/nebulagc-server/nebulagc.db*

# 3. Restore from replica
sudo -u nebulagc-server litestream restore -config /etc/litestream.yml \
  /var/lib/nebulagc-server/nebulagc.db

# 4. Restart services
sudo systemctl start litestream
sudo systemctl start nebulagc-server
```

### Scenario 2: Migrate to New Server

```bash
# On new server:

# 1. Install Litestream and configure
sudo cp /etc/litestream.yml /etc/litestream.yml

# 2. Restore database
sudo -u nebulagc-server litestream restore -config /etc/litestream.yml \
  /var/lib/nebulagc-server/nebulagc.db

# 3. Start services
sudo systemctl start litestream
sudo systemctl start nebulagc-server

# Old server will continue replicating (same generation)
# New server will read from that generation
```

### Scenario 3: Accidental Data Loss

```bash
# Restore to 1 hour ago
sudo systemctl stop nebulagc-server
sudo systemctl stop litestream

# Backup current database
sudo mv /var/lib/nebulagc-server/nebulagc.db /var/lib/nebulagc-server/nebulagc.db.broken

# Restore to timestamp
sudo -u nebulagc-server litestream restore -config /etc/litestream.yml \
  -timestamp "$(date -u -d '1 hour ago' '+%Y-%m-%dT%H:%M:%SZ')" \
  /var/lib/nebulagc-server/nebulagc.db

# Restart
sudo systemctl start litestream
sudo systemctl start nebulagc-server
```

---

## Best Practices

1. **Enable WAL Mode**: Litestream requires SQLite WAL mode
   ```sql
   PRAGMA journal_mode=WAL;
   ```

2. **Configure Retention**: Balance cost vs recovery window
   ```yaml
   retention: 168h  # Keep 7 days of snapshots
   retention-check-interval: 1h
   ```

3. **Multiple Replicas**: Replicate to multiple regions
   ```yaml
   replicas:
     - url: s3://us-bucket/nebulagc/db
       region: us-east-1
     - url: s3://eu-bucket/nebulagc/db
       region: eu-west-1
   ```

4. **Test Restores**: Regularly test restoration process
   ```bash
   # Monthly restore test
   litestream restore -config /etc/litestream.yml /tmp/test-restore.db
   sqlite3 /tmp/test-restore.db "PRAGMA integrity_check;"
   ```

5. **Monitor Lag**: Alert on excessive lag
   ```bash
   LAG=$(litestream replicas ... | tail -n1 | awk '{print $3}' | sed 's/s//')
   if [ "$LAG" -gt 10 ]; then
     echo "WARNING: Replication lag is ${LAG}s"
   fi
   ```

6. **Snapshot Interval**: Balance storage vs recovery time
   ```yaml
   snapshot-interval: 6h  # Fewer snapshots, but longer recovery
   ```

7. **Validation**: Periodically validate replica integrity
   ```yaml
   validation-interval: 24h
   ```

8. **Encryption**: Use cloud provider encryption
   - S3: Enable SSE-S3 or SSE-KMS
   - GCS: Enable customer-managed encryption
   - Azure: Enable blob encryption

9. **Access Control**: Restrict S3 bucket access
   ```json
   {
     "Version": "2012-10-17",
     "Statement": [{
       "Effect": "Allow",
       "Action": ["s3:*"],
       "Resource": [
         "arn:aws:s3:::my-bucket/nebulagc/*"
       ]
     }]
   }
   ```

10. **Lifecycle Policies**: Automatically delete old backups
    ```xml
    <LifecycleConfiguration>
      <Rule>
        <Filter>
          <Prefix>nebulagc/</Prefix>
        </Filter>
        <Status>Enabled</Status>
        <Expiration>
          <Days>30</Days>
        </Expiration>
      </Rule>
    </LifecycleConfiguration>
    ```

---

## Troubleshooting

### Replication Not Working

```bash
# Check Litestream logs
sudo journalctl -u litestream -f

# Verify database is in WAL mode
sqlite3 /var/lib/nebulagc-server/nebulagc.db "PRAGMA journal_mode;"

# Check S3 permissions
aws s3 ls s3://my-bucket/nebulagc/

# Verify config syntax
litestream replicate -config /etc/litestream.yml -verbose
```

### High Lag

```bash
# Check network latency
time aws s3 ls s3://my-bucket/

# Monitor sync operations
litestream replicate -config /etc/litestream.yml -verbose

# Reduce sync interval (more API calls)
# Edit /etc/litestream.yml: sync-interval: 500ms
```

### Restore Fails

```bash
# List available generations
litestream generations -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db

# Try restoring specific generation
litestream restore -config /etc/litestream.yml \
  -generation <generation-id> \
  /var/lib/nebulagc-server/nebulagc.db

# Check replica integrity
litestream snapshots -config /etc/litestream.yml /var/lib/nebulagc-server/nebulagc.db
```

### Permission Errors

```bash
# Check file ownership
ls -la /var/lib/nebulagc-server/nebulagc.db*

# Fix ownership
sudo chown nebulagc-server:nebulagc-server /var/lib/nebulagc-server/nebulagc.db*

# Check Litestream user
ps aux | grep litestream
```

---

## Cost Estimation

### AWS S3

- **Storage**: ~$0.023/GB/month (Standard)
- **PUT Requests**: $0.005/1000 requests
- **GET Requests**: $0.0004/1000 requests

Example: 10GB database with moderate writes:
- Storage: 10GB × $0.023 = **$0.23/month**
- WAL uploads: ~100k/month × $0.000005 = **$0.50/month**
- Total: **~$0.75/month**

### Google Cloud Storage

- **Storage**: ~$0.020/GB/month (Standard)
- **Operations**: Class A (writes) $0.05/10k, Class B (reads) $0.004/10k

### Azure Blob Storage

- **Storage**: ~$0.0184/GB/month (Hot tier)
- **Operations**: Write $0.05/10k, Read $0.0004/10k

---

## Additional Resources

- [Litestream Official Documentation](https://litestream.io/)
- [Litestream GitHub](https://github.com/benbjohnson/litestream)
- [SQLite WAL Mode](https://www.sqlite.org/wal.html)
- [AWS S3 Documentation](https://docs.aws.amazon.com/s3/)
- [Disaster Recovery Best Practices](https://litestream.io/guides/disaster-recovery/)
