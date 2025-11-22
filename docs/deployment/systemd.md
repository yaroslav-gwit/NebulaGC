# Systemd Deployment Guide

This guide covers deploying NebulaGC Server and Daemon as systemd services on Linux systems.

---

## Overview

Systemd provides reliable process management with automatic restarts, logging, and service dependencies. This guide includes:

- Systemd service files for server and daemon
- Environment variable configuration
- Security hardening
- Log management with journald
- Installation and verification steps

---

## Prerequisites

- Linux system with systemd (Ubuntu 16.04+, Debian 8+, RHEL 7+, CentOS 7+)
- Root or sudo access
- NebulaGC binaries (`nebulagc-server` and `nebulagc` daemon)

---

## Server Deployment

### 1. Install Server Binary

```bash
# Create installation directory
sudo mkdir -p /usr/local/bin

# Copy server binary
sudo cp nebulagc-server /usr/local/bin/
sudo chmod +x /usr/local/bin/nebulagc-server

# Verify installation
/usr/local/bin/nebulagc-server --version
```

### 2. Create Service User

```bash
# Create dedicated user for security isolation
sudo useradd --system --no-create-home --shell /bin/false nebulagc-server
```

### 3. Create Data Directories

```bash
# Create database and configuration directories
sudo mkdir -p /var/lib/nebulagc-server
sudo mkdir -p /etc/nebulagc

# Set ownership
sudo chown nebulagc-server:nebulagc-server /var/lib/nebulagc-server
sudo chown root:nebulagc-server /etc/nebulagc
sudo chmod 750 /etc/nebulagc
```

### 4. Create Environment File

Create `/etc/nebulagc/server.env`:

```bash
# Database configuration
NEBULAGC_DB_PATH=/var/lib/nebulagc-server/nebulagc.db

# Server configuration
NEBULAGC_LISTEN_ADDR=0.0.0.0:8080
NEBULAGC_HMAC_SECRET=your-secret-key-here-change-me

# High availability (optional)
NEBULAGC_HA_MODE=master
# NEBULAGC_REPLICA_URLS=http://replica1:8080,http://replica2:8080

# Logging
NEBULAGC_LOG_LEVEL=info
NEBULAGC_LOG_FORMAT=json
NEBULAGC_LOG_SAMPLING=true

# Rate limiting
NEBULAGC_RATELIMIT_AUTH_FAILURES_PER_MIN=10
NEBULAGC_RATELIMIT_REQUESTS_PER_MIN=100

# TLS (optional)
# NEBULAGC_TLS_CERT=/etc/nebulagc/tls/server.crt
# NEBULAGC_TLS_KEY=/etc/nebulagc/tls/server.key
```

Set secure permissions:

```bash
sudo chown root:nebulagc-server /etc/nebulagc/server.env
sudo chmod 640 /etc/nebulagc/server.env
```

### 5. Create Systemd Service File

Create `/etc/systemd/system/nebulagc-server.service`:

```ini
[Unit]
Description=NebulaGC Control Plane Server
Documentation=https://github.com/yaroslav-gwit/NebulaGC
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nebulagc-server
Group=nebulagc-server

# Environment
EnvironmentFile=/etc/nebulagc/server.env

# Process execution
ExecStart=/usr/local/bin/nebulagc-server
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nebulagc-server
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
SystemCallArchitectures=native

# Resource limits
LimitNOFILE=65536
LimitNPROC=512

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nebulagc-server

[Install]
WantedBy=multi-user.target
```

### 6. Enable and Start Service

```bash
# Reload systemd daemon
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable nebulagc-server

# Start service
sudo systemctl start nebulagc-server

# Check status
sudo systemctl status nebulagc-server
```

### 7. Verify Server is Running

```bash
# Check service status
sudo systemctl status nebulagc-server

# View logs
sudo journalctl -u nebulagc-server -f

# Test health endpoint
curl http://localhost:8080/health
```

Expected output:
```json
{"status":"healthy","timestamp":"2024-11-21T10:00:00Z"}
```

---

## Daemon Deployment

### 1. Install Daemon Binary

```bash
# Copy daemon binary
sudo cp nebulagc /usr/local/bin/
sudo chmod +x /usr/local/bin/nebulagc

# Verify installation
/usr/local/bin/nebulagc --version
```

### 2. Create Service User

```bash
# Create dedicated user (if not exists)
sudo useradd --system --no-create-home --shell /bin/false nebulagc-daemon
```

### 3. Create Configuration Directory

```bash
# Create configuration directory
sudo mkdir -p /etc/nebulagc/daemon
sudo mkdir -p /var/lib/nebulagc-daemon

# Set ownership
sudo chown nebulagc-daemon:nebulagc-daemon /var/lib/nebulagc-daemon
sudo chown root:nebulagc-daemon /etc/nebulagc/daemon
sudo chmod 750 /etc/nebulagc/daemon
```

### 4. Create Daemon Configuration

Create `/etc/nebulagc/daemon/config.yaml`:

```yaml
# Control plane URLs
control_plane_urls:
  - "http://control-plane-1.example.com:8080"
  - "http://control-plane-2.example.com:8080"
  - "http://control-plane-3.example.com:8080"

# Clusters managed by this daemon
clusters:
  - cluster_id: "cluster-1"
    node_id: "node-1"
    node_token: "your-node-token-here"
    config_dir: "/etc/nebula/cluster-1"
    provide_lighthouse: false

  - cluster_id: "cluster-2"
    node_id: "node-2"
    node_token: "your-node-token-here"
    config_dir: "/etc/nebula/cluster-2"
    provide_lighthouse: true

# Poll interval (default: 5 seconds)
poll_interval: 5s

# Process management
nebula_binary: "/usr/local/bin/nebula"
state_dir: "/var/lib/nebulagc-daemon"
```

Set secure permissions:

```bash
sudo chown root:nebulagc-daemon /etc/nebulagc/daemon/config.yaml
sudo chmod 640 /etc/nebulagc/daemon/config.yaml
```

### 5. Create Systemd Service File

Create `/etc/systemd/system/nebulagc-daemon.service`:

```ini
[Unit]
Description=NebulaGC Daemon - Nebula Configuration Manager
Documentation=https://github.com/yaroslav-gwit/NebulaGC
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=nebulagc-daemon
Group=nebulagc-daemon

# Process execution
ExecStart=/usr/local/bin/nebulagc daemon --config /etc/nebulagc/daemon/config.yaml
Restart=always
RestartSec=5

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nebulagc-daemon /etc/nebula
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
RestrictSUIDSGID=true
LockPersonality=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
SystemCallArchitectures=native

# Allow daemon to manage Nebula processes
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_RAW

# Resource limits
LimitNOFILE=65536
LimitNPROC=512

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nebulagc-daemon

[Install]
WantedBy=multi-user.target
```

### 6. Enable and Start Service

```bash
# Reload systemd daemon
sudo systemctl daemon-reload

# Enable service to start on boot
sudo systemctl enable nebulagc-daemon

# Start service
sudo systemctl start nebulagc-daemon

# Check status
sudo systemctl status nebulagc-daemon
```

### 7. Verify Daemon is Running

```bash
# Check service status
sudo systemctl status nebulagc-daemon

# View logs
sudo journalctl -u nebulagc-daemon -f

# Check Nebula processes
ps aux | grep nebula
```

---

## Log Management

### Viewing Logs

```bash
# View server logs (last 100 lines)
sudo journalctl -u nebulagc-server -n 100

# View daemon logs (last 100 lines)
sudo journalctl -u nebulagc-daemon -n 100

# Follow logs in real-time
sudo journalctl -u nebulagc-server -f

# View logs since last boot
sudo journalctl -u nebulagc-server -b

# View logs for specific time range
sudo journalctl -u nebulagc-server --since "2024-11-20 10:00:00" --until "2024-11-20 11:00:00"

# View logs with priority
sudo journalctl -u nebulagc-server -p err
```

### Log Retention

Configure journald retention in `/etc/systemd/journald.conf`:

```ini
[Journal]
# Keep logs for 30 days
MaxRetentionSec=30d

# Limit disk usage to 1GB
SystemMaxUse=1G

# Compress logs older than 1 day
Compress=yes
```

Restart journald after changes:

```bash
sudo systemctl restart systemd-journald
```

---

## Security Hardening

### 1. Generate Secure HMAC Secret

```bash
# Generate random 32-byte HMAC secret
openssl rand -hex 32
```

Update `/etc/nebulagc/server.env` with the generated secret.

### 2. TLS Configuration (Recommended for Production)

Generate self-signed certificate (for testing):

```bash
sudo mkdir -p /etc/nebulagc/tls

# Generate private key
sudo openssl genrsa -out /etc/nebulagc/tls/server.key 4096

# Generate certificate (valid for 1 year)
sudo openssl req -new -x509 -key /etc/nebulagc/tls/server.key \
  -out /etc/nebulagc/tls/server.crt -days 365 \
  -subj "/CN=nebulagc-server.example.com"

# Set secure permissions
sudo chown root:nebulagc-server /etc/nebulagc/tls/server.key
sudo chmod 640 /etc/nebulagc/tls/server.key
```

Update `/etc/nebulagc/server.env`:

```bash
NEBULAGC_TLS_CERT=/etc/nebulagc/tls/server.crt
NEBULAGC_TLS_KEY=/etc/nebulagc/tls/server.key
```

Restart server:

```bash
sudo systemctl restart nebulagc-server
```

### 3. Firewall Configuration

```bash
# Allow server port (HTTP)
sudo ufw allow 8080/tcp

# Or for HTTPS
sudo ufw allow 8443/tcp

# Enable firewall
sudo ufw enable
```

### 4. File Permissions Review

```bash
# Verify permissions
ls -la /etc/nebulagc/
ls -la /var/lib/nebulagc-server/
ls -la /var/lib/nebulagc-daemon/

# Database should be readable/writable only by service user
sudo chmod 600 /var/lib/nebulagc-server/nebulagc.db

# Environment files should not be world-readable
sudo chmod 640 /etc/nebulagc/server.env
sudo chmod 640 /etc/nebulagc/daemon/config.yaml
```

---

## Service Management

### Starting/Stopping Services

```bash
# Start services
sudo systemctl start nebulagc-server
sudo systemctl start nebulagc-daemon

# Stop services
sudo systemctl stop nebulagc-server
sudo systemctl stop nebulagc-daemon

# Restart services
sudo systemctl restart nebulagc-server
sudo systemctl restart nebulagc-daemon

# Reload configuration (if supported)
sudo systemctl reload nebulagc-server
```

### Enable/Disable Auto-Start

```bash
# Enable auto-start on boot
sudo systemctl enable nebulagc-server
sudo systemctl enable nebulagc-daemon

# Disable auto-start
sudo systemctl disable nebulagc-server
sudo systemctl disable nebulagc-daemon
```

### Check Service Status

```bash
# Detailed status
sudo systemctl status nebulagc-server
sudo systemctl status nebulagc-daemon

# Check if service is active
systemctl is-active nebulagc-server

# Check if service is enabled
systemctl is-enabled nebulagc-server
```

---

## Troubleshooting

### Service Won't Start

```bash
# Check service status for errors
sudo systemctl status nebulagc-server

# View detailed logs
sudo journalctl -u nebulagc-server -n 100 --no-pager

# Check configuration file syntax
/usr/local/bin/nebulagc-server --help

# Verify environment file
sudo cat /etc/nebulagc/server.env

# Check file permissions
ls -la /var/lib/nebulagc-server/
```

### Database Lock Errors

```bash
# Check if another process is using the database
sudo lsof /var/lib/nebulagc-server/nebulagc.db

# Verify WAL mode is enabled
sudo sqlite3 /var/lib/nebulagc-server/nebulagc.db "PRAGMA journal_mode;"
```

### Permission Denied Errors

```bash
# Check service user
id nebulagc-server

# Verify directory ownership
ls -ld /var/lib/nebulagc-server/
ls -la /var/lib/nebulagc-server/

# Fix ownership if needed
sudo chown -R nebulagc-server:nebulagc-server /var/lib/nebulagc-server/
```

### High Memory Usage

```bash
# Check memory usage
sudo systemctl status nebulagc-server

# View process details
ps aux | grep nebulagc-server

# Add memory limits to service file
sudo systemctl edit nebulagc-server
```

Add to override file:

```ini
[Service]
MemoryMax=512M
MemoryHigh=384M
```

### Service Crashes/Restarts

```bash
# View crash logs
sudo journalctl -u nebulagc-server -p err

# Check restart count
systemctl show nebulagc-server -p NRestarts

# Review last crash dump (if coredumps enabled)
coredumpctl list nebulagc-server
coredumpctl info <crash-id>
```

---

## Upgrade Procedure

```bash
# Stop service
sudo systemctl stop nebulagc-server

# Backup database
sudo cp /var/lib/nebulagc-server/nebulagc.db /var/lib/nebulagc-server/nebulagc.db.backup

# Replace binary
sudo cp nebulagc-server /usr/local/bin/
sudo chmod +x /usr/local/bin/nebulagc-server

# Verify version
/usr/local/bin/nebulagc-server --version

# Start service
sudo systemctl start nebulagc-server

# Check logs for errors
sudo journalctl -u nebulagc-server -f
```

---

## Monitoring

### Prometheus Metrics

Server exposes metrics at `/metrics` endpoint:

```bash
curl http://localhost:8080/metrics
```

Configure Prometheus to scrape:

```yaml
scrape_configs:
  - job_name: 'nebulagc-server'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### Health Checks

```bash
# Server health
curl http://localhost:8080/health

# With timeout
curl --max-time 5 http://localhost:8080/health || echo "Health check failed"
```

### Watchdog Integration

Add watchdog to systemd service (optional):

```ini
[Service]
WatchdogSec=30
```

Application must call `sd_notify(0, "WATCHDOG=1")` periodically.

---

## Best Practices

1. **Use Dedicated Service Users**: Never run as root
2. **Enable Security Hardening**: Use all systemd security features
3. **Configure Log Retention**: Prevent disk space issues
4. **Monitor Services**: Set up alerts for service failures
5. **Regular Backups**: Backup database before upgrades
6. **Use TLS in Production**: Encrypt control plane traffic
7. **Rotate Secrets**: Periodically rotate HMAC secret and tokens
8. **Resource Limits**: Set appropriate memory and CPU limits
9. **Test Restarts**: Verify service restarts cleanly
10. **Document Changes**: Keep track of configuration changes

---

## Additional Resources

- [Systemd Service Documentation](https://www.freedesktop.org/software/systemd/man/systemd.service.html)
- [Systemd Security Hardening](https://www.freedesktop.org/software/systemd/man/systemd.exec.html#Sandboxing)
- [Journald Configuration](https://www.freedesktop.org/software/systemd/man/journald.conf.html)
- [NebulaGC Configuration Reference](../configuration.md)
