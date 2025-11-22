# NebulaGC Security Checklist

**Document Version**: 1.0  
**Date**: 2025-11-22  
**Purpose**: Production deployment security checklist

---

## Pre-Deployment Checklist

### 1. Server Configuration

#### Required ✓
- [ ] **HMAC Secret**: Generate strong random secret (32+ bytes)
  ```bash
  openssl rand -hex 32
  ```
- [ ] **Instance ID**: Set unique UUID per instance
- [ ] **Public URL**: Configure externally accessible URL
- [ ] **HA Mode**: Specify master or replica mode
- [ ] **Database Path**: Set secure location with proper permissions

#### Recommended ✓
- [ ] **TLS Enabled**: Configure TLS certificates
  - Use valid certificates from trusted CA
  - Or use Let's Encrypt for public deployments
- [ ] **Log Level**: Set to `info` for production (not `debug`)
- [ ] **Log Format**: Use `json` for structured logging
- [ ] **CORS Origins**: Restrict to known origins (not `*`)

### 2. Filesystem Security

#### Required ✓
- [ ] **Database Permissions**: Set to 600 (owner read/write only)
  ```bash
  chmod 600 /path/to/nebula.db
  ```
- [ ] **Config File Permissions**: Set to 600
  ```bash
  chmod 600 /etc/nebulagc/config.yaml
  ```
- [ ] **Binary Ownership**: Set appropriate user/group
  ```bash
  chown nebulagc:nebulagc /usr/local/bin/nebulagc-server
  chmod 755 /usr/local/bin/nebulagc-server
  ```
- [ ] **Directory Permissions**: Restrict config/data directories
  ```bash
  chmod 700 /var/lib/nebulagc
  chmod 700 /etc/nebulagc
  ```

#### Recommended ✓
- [ ] **Dedicated User**: Run server as non-root user
- [ ] **Dedicated Group**: Use dedicated group for permissions
- [ ] **SELinux/AppArmor**: Configure mandatory access control

### 3. Network Security

#### Required ✓
- [ ] **Firewall Rules**: Restrict access to control plane port
  ```bash
  # Allow only from daemon subnet
  iptables -A INPUT -p tcp --dport 8080 -s 10.0.0.0/8 -j ACCEPT
  iptables -A INPUT -p tcp --dport 8080 -j DROP
  ```
- [ ] **TLS Configuration**: Use TLS 1.2+ only
- [ ] **Certificate Validation**: Verify certificate chain

#### Recommended ✓
- [ ] **Private Network**: Deploy control plane on private network
- [ ] **VPN/Bastion**: Access server only through VPN or bastion host
- [ ] **Network Segmentation**: Isolate control plane from untrusted networks
- [ ] **DDoS Protection**: Use cloud provider DDoS protection

### 4. Authentication & Authorization

#### Required ✓
- [ ] **Token Generation**: Use crypto/rand for all tokens
- [ ] **Token Length**: Minimum 32 bytes (256 bits)
- [ ] **Bcrypt Cost**: Use cost >= 12 for password hashing
- [ ] **Token Storage**: Store tokens hashed (never plaintext in DB)
- [ ] **HMAC Secret Protection**: Store only in environment variables

#### Recommended ✓
- [ ] **Token Rotation**: Implement periodic token rotation
- [ ] **Token Expiration**: Set token TTL (future enhancement)
- [ ] **Multi-Factor Auth**: Add MFA for administrative access (future)

### 5. Database Security

#### Required ✓
- [ ] **File Permissions**: Database file is 600
- [ ] **Backups**: Regular automated backups
- [ ] **Backup Testing**: Verify backups can be restored
- [ ] **WAL Mode**: Use Write-Ahead Logging for concurrency
  ```sql
  PRAGMA journal_mode=WAL;
  ```

#### Recommended ✓
- [ ] **Backup Encryption**: Encrypt backup files
- [ ] **Off-site Backups**: Store backups in separate location
- [ ] **Backup Retention**: Define and enforce retention policy
- [ ] **Database Encryption**: Encrypt database at rest (future)

### 6. Logging & Monitoring

#### Required ✓
- [ ] **Structured Logging**: Enable JSON logging
- [ ] **Log Rotation**: Configure log rotation (logrotate)
- [ ] **No Sensitive Data**: Verify tokens/secrets not logged
- [ ] **Authentication Logging**: Log all auth attempts

#### Recommended ✓
- [ ] **Centralized Logging**: Send logs to SIEM/log aggregator
- [ ] **Alerting**: Configure alerts for security events
  - Repeated authentication failures
  - High error rates
  - Resource exhaustion
- [ ] **Metrics**: Monitor Prometheus metrics
- [ ] **Audit Trail**: Immutable audit log for compliance

### 7. Rate Limiting

#### Required ✓
- [ ] **Auth Failure Limits**: Configured (default: 10/min)
- [ ] **Request Limits**: Configured (default: 100/min)
- [ ] **Bundle Upload Limits**: Configured (default: 10/min)
- [ ] **Health Check Limits**: Configured (default: 30/min)

#### Recommended ✓
- [ ] **Per-IP Limits**: Enforce per-IP rate limits
- [ ] **Per-Node Limits**: Enforce per-node rate limits
- [ ] **Backoff Strategy**: Implement exponential backoff

### 8. Process Isolation

#### Required ✓
- [ ] **Non-Root User**: Server runs as non-root
- [ ] **Minimal Privileges**: Drop unnecessary capabilities
- [ ] **Resource Limits**: Set ulimits for server process
  ```bash
  # In systemd service file
  LimitNOFILE=65536
  LimitNPROC=512
  ```

#### Recommended ✓
- [ ] **Container Isolation**: Run in container (Docker/Kubernetes)
- [ ] **Read-Only Filesystem**: Mount most directories read-only
- [ ] **Seccomp Profile**: Apply seccomp filter
- [ ] **Namespace Isolation**: Use Linux namespaces

### 9. High Availability

#### Required ✓
- [ ] **Multiple Instances**: Deploy 3+ replicas for HA
- [ ] **Health Checks**: Configure liveness/readiness probes
- [ ] **Replica Sync**: Verify replica synchronization
- [ ] **Failover Testing**: Test failover scenarios

#### Recommended ✓
- [ ] **Geographic Distribution**: Distribute replicas across regions
- [ ] **Automated Failover**: Configure automatic failover
- [ ] **Backup Master**: Maintain hot standby master
- [ ] **Load Balancing**: Use load balancer for replica traffic

### 10. Operational Security

#### Required ✓
- [ ] **Security Updates**: Subscribe to security announcements
- [ ] **Patch Management**: Apply security patches promptly
- [ ] **Vulnerability Scanning**: Regular vulnerability scans
- [ ] **Dependency Updates**: Keep dependencies up-to-date

#### Recommended ✓
- [ ] **Incident Response Plan**: Document incident response
- [ ] **Security Training**: Train operators on security
- [ ] **Change Control**: Require approval for changes
- [ ] **Disaster Recovery**: Test disaster recovery procedures

---

## Post-Deployment Verification

### Security Testing

- [ ] **Penetration Test**: Conduct external penetration test
- [ ] **Vulnerability Scan**: Run automated vulnerability scanner
- [ ] **SSL/TLS Test**: Verify TLS configuration (ssllabs.com)
- [ ] **Auth Testing**: Test authentication bypass attempts
- [ ] **Input Fuzzing**: Fuzz API endpoints for injection vulnerabilities

### Monitoring Setup

- [ ] **Metrics Dashboard**: Grafana/Prometheus dashboard
- [ ] **Log Aggregation**: ELK/Splunk/CloudWatch setup
- [ ] **Alert Rules**: Configure alerting rules
- [ ] **On-Call Rotation**: Establish on-call rotation
- [ ] **Runbook**: Document operational procedures

### Compliance

- [ ] **Security Policy**: Document security policies
- [ ] **Audit Trail**: Verify audit logging
- [ ] **Data Retention**: Implement retention policies
- [ ] **Access Control**: Document and enforce access control

---

## Environment-Specific Checklists

### Development Environment

**Security can be relaxed but must include**:
- [ ] Different HMAC secret than production
- [ ] Separate database (no production data)
- [ ] Self-signed certificates acceptable
- [ ] Logging in debug mode acceptable
- [ ] No real tokens/credentials

### Staging Environment

**Should mirror production security**:
- [ ] Same security controls as production
- [ ] Production-like TLS certificates
- [ ] Realistic data (but not production data)
- [ ] Full monitoring and alerting
- [ ] Test incident response procedures

### Production Environment

**All security controls required**:
- [ ] Complete this checklist ✓
- [ ] Security audit performed
- [ ] Change control process
- [ ] Incident response plan
- [ ] Regular security reviews

---

## Deployment Patterns

### Docker Deployment

```bash
# Required configurations
docker run -d \
  --name nebulagc-server \
  --user 1000:1000 \
  --read-only \
  --tmpfs /tmp:noexec,nosuid,nodev \
  --security-opt=no-new-privileges:true \
  --cap-drop=ALL \
  --cap-add=NET_BIND_SERVICE \
  -v /data/nebula.db:/data/nebula.db:rw \
  -e NEBULAGC_HMAC_SECRET="$(openssl rand -hex 32)" \
  -e NEBULAGC_DB_PATH=/data/nebula.db \
  -p 8080:8080 \
  nebulagc-server:latest
```

### Kubernetes Deployment

```yaml
# Security context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  allowPrivilegeEscalation: false
  capabilities:
    drop: ["ALL"]
  readOnlyRootFilesystem: true
  seccompProfile:
    type: RuntimeDefault

# Pod security policy
podsecuritypolicy:
  enabled: true
  privileged: false
  allowPrivilegeEscalation: false
  runAsUser: MustRunAsNonRoot
```

### Systemd Deployment

```ini
[Unit]
Description=NebulaGC Control Plane

[Service]
Type=simple
User=nebulagc
Group=nebulagc
ExecStart=/usr/local/bin/nebulagc-server
Restart=always
RestartSec=10

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nebulagc
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

---

## Security Incident Checklist

### Detection
- [ ] Security event detected
- [ ] Incident logged with timestamp
- [ ] Severity assessed (Critical/High/Medium/Low)
- [ ] Incident team notified

### Containment
- [ ] Affected systems isolated
- [ ] Access revoked for compromised accounts
- [ ] Network segmentation enforced
- [ ] Logs preserved for analysis

### Investigation
- [ ] Audit logs reviewed
- [ ] Attack vector identified
- [ ] Scope of compromise determined
- [ ] Affected data/systems catalogued

### Remediation
- [ ] Vulnerabilities patched
- [ ] Compromised credentials rotated
- [ ] Systems restored from clean backups
- [ ] Security controls strengthened

### Post-Incident
- [ ] Root cause analysis completed
- [ ] Post-mortem document created
- [ ] Lessons learned documented
- [ ] Security controls updated
- [ ] Incident response plan updated

---

## Checklist Maintenance

This checklist should be reviewed:
- Before each deployment
- After security incidents
- Quarterly as part of security review
- When new features are added

**Last Updated**: 2025-11-22  
**Next Review**: 2026-02-22
